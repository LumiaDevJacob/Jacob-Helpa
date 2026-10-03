package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// The vault keeps short notes encrypted on disk with AES-256-GCM. The key is
// derived from the master password with PBKDF2-HMAC-SHA256; the password itself
// is never stored, so forgetting it means the notes are gone.
const (
	vaultMagic      = "JHV1"
	vaultSaltLen    = 16
	vaultKeyLen     = 32
	vaultIterations = 600_000 // OWASP's 2023 floor for PBKDF2-HMAC-SHA256
)

var errLocked = errors.New("the vault is locked")

type note struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Updated string `json:"updated"`
}

type vault struct {
	path string

	mu    sync.Mutex
	key   []byte // nil while locked
	salt  []byte
	notes []note
}

func newVault(path string) *vault {
	return &vault{path: path}
}

func (v *vault) exists() bool {
	info, err := os.Stat(v.path)
	return err == nil && info.Size() > 0
}

func (v *vault) isUnlocked() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.key != nil
}

func (v *vault) lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	// Wipe the key material rather than just dropping the reference.
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
	v.notes = nil
}

// unlock derives the key and decrypts the store. For a vault that doesn't exist
// yet this creates an empty one with the given password.
func (v *vault) unlock(password string) error {
	if len(password) < 8 {
		return errors.New("the master password needs at least 8 characters")
	}

	if !v.exists() {
		salt := make([]byte, vaultSaltLen)
		if _, err := rand.Read(salt); err != nil {
			return fmt.Errorf("no secure randomness available: %w", err)
		}
		key, err := deriveKey(password, salt)
		if err != nil {
			return err
		}
		v.mu.Lock()
		v.key, v.salt, v.notes = key, salt, nil
		v.mu.Unlock()
		return v.persist()
	}

	blob, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Errorf("could not read the vault: %w", err)
	}
	header := len(vaultMagic) + vaultSaltLen
	if len(blob) < header || string(blob[:len(vaultMagic)]) != vaultMagic {
		return errors.New("that vault file isn't one Jacob Helpa wrote")
	}
	salt := blob[len(vaultMagic):header]
	body := blob[header:]

	key, err := deriveKey(password, salt)
	if err != nil {
		return err
	}
	plain, err := decrypt(key, body)
	if err != nil {
		return errors.New("wrong master password")
	}

	var notes []note
	if len(plain) > 0 {
		if err := json.Unmarshal(plain, &notes); err != nil {
			return errors.New("the vault contents are corrupt")
		}
	}

	v.mu.Lock()
	v.key, v.salt, v.notes = key, append([]byte(nil), salt...), notes
	v.mu.Unlock()
	return nil
}

// persist re-encrypts and writes the whole store. A fresh nonce is used on
// every write, which GCM requires.
func (v *vault) persist() error {
	v.mu.Lock()
	if v.key == nil {
		v.mu.Unlock()
		return errLocked
	}
	key := append([]byte(nil), v.key...)
	salt := append([]byte(nil), v.salt...)
	plain, err := json.Marshal(v.notes)
	v.mu.Unlock()
	if err != nil {
		return err
	}

	sealed, err := encrypt(key, plain)
	if err != nil {
		return err
	}

	out := make([]byte, 0, len(vaultMagic)+len(salt)+len(sealed))
	out = append(out, vaultMagic...)
	out = append(out, salt...)
	out = append(out, sealed...)

	tmp := v.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("could not write the vault: %w", err)
	}
	if err := os.Rename(tmp, v.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("could not save the vault: %w", err)
	}
	return nil
}

func deriveKey(password string, salt []byte) ([]byte, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, vaultIterations, vaultKeyLen)
	if err != nil {
		return nil, fmt.Errorf("could not derive a key: %w", err)
	}
	return key, nil
}

func encrypt(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("no secure randomness available: %w", err)
	}
	// Seal appends the ciphertext to the nonce, so the nonce travels with it.
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decrypt(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, body := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

func (v *vault) list() ([]note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return nil, errLocked
	}
	out := append([]note(nil), v.notes...)
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out, nil
}

func (v *vault) save(n note) (note, error) {
	n.Title = strings.TrimSpace(n.Title)
	if n.Title == "" {
		return note{}, errors.New("give the note a title")
	}
	if len(n.Title) > 120 {
		return note{}, errors.New("that title is too long")
	}
	if len(n.Body) > 100_000 {
		return note{}, errors.New("that note is too long")
	}
	n.Updated = time.Now().Format(time.RFC3339)

	v.mu.Lock()
	if v.key == nil {
		v.mu.Unlock()
		return note{}, errLocked
	}
	if n.ID == "" {
		id, err := uuidV4()
		if err != nil {
			v.mu.Unlock()
			return note{}, err
		}
		n.ID = id
		v.notes = append(v.notes, n)
	} else {
		found := false
		for i := range v.notes {
			if subtle.ConstantTimeCompare([]byte(v.notes[i].ID), []byte(n.ID)) == 1 {
				v.notes[i] = n
				found = true
				break
			}
		}
		if !found {
			v.mu.Unlock()
			return note{}, errors.New("that note no longer exists")
		}
	}
	v.mu.Unlock()

	if err := v.persist(); err != nil {
		return note{}, err
	}
	return n, nil
}

func (v *vault) remove(id string) error {
	v.mu.Lock()
	if v.key == nil {
		v.mu.Unlock()
		return errLocked
	}
	kept := make([]note, 0, len(v.notes))
	removed := false
	for _, n := range v.notes {
		if n.ID == id {
			removed = true
			continue
		}
		kept = append(kept, n)
	}
	v.notes = kept
	v.mu.Unlock()

	if !removed {
		return errors.New("that note no longer exists")
	}
	return v.persist()
}

// --- handlers ------------------------------------------------------------

func (a *app) handleVaultStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"exists":   a.vault.exists(),
		"unlocked": a.vault.isUnlocked(),
	})
}

func (a *app) handleVaultUnlock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := a.vault.unlock(req.Password); err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	notes, err := a.vault.list()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": true, "notes": notes})
}

func (a *app) handleVaultLock(w http.ResponseWriter, r *http.Request) {
	a.vault.lock()
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": false})
}

func (a *app) handleVaultList(w http.ResponseWriter, r *http.Request) {
	notes, err := a.vault.list()
	if err != nil {
		writeErr(w, http.StatusLocked, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

func (a *app) handleVaultSave(w http.ResponseWriter, r *http.Request) {
	var n note
	if !readJSON(w, r, &n) {
		return
	}
	saved, err := a.vault.save(n)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errLocked) {
			status = http.StatusLocked
		}
		writeErr(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (a *app) handleVaultDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := a.vault.remove(req.ID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errLocked) {
			status = http.StatusLocked
		}
		writeErr(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
