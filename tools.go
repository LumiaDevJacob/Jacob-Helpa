package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars  = "0123456789"
	symbolChars = "!@#$%^&*()-_=+[]{};:,.?/"
	// Characters that are easy to misread when typing a password by hand.
	ambiguousChars = "O0oIl1|S5B8G6Z2"
)

// pick returns one uniformly random rune from set using crypto/rand, so the
// output is suitable for passwords and tokens.
func pick(set []rune) (rune, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, fmt.Errorf("no secure randomness available: %w", err)
	}
	return set[n.Int64()], nil
}

// shuffle does a crypto/rand Fisher-Yates shuffle in place.
func shuffle(runes []rune) error {
	for i := len(runes) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("no secure randomness available: %w", err)
		}
		j := n.Int64()
		runes[i], runes[j] = runes[j], runes[i]
	}
	return nil
}

type passwordRequest struct {
	Length      int  `json:"length"`
	Count       int  `json:"count"`
	Lower       bool `json:"lower"`
	Upper       bool `json:"upper"`
	Digits      bool `json:"digits"`
	Symbols     bool `json:"symbols"`
	NoAmbiguous bool `json:"noAmbiguous"`
}

func (a *app) handlePassword(w http.ResponseWriter, r *http.Request) {
	req := passwordRequest{Length: 20, Count: 5, Lower: true, Upper: true, Digits: true, Symbols: true}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Length < 4 || req.Length > 256 {
		writeErr(w, http.StatusBadRequest, "length must be between 4 and 256")
		return
	}
	if req.Count < 1 || req.Count > 50 {
		writeErr(w, http.StatusBadRequest, "count must be between 1 and 50")
		return
	}

	// Each enabled class becomes a pool; one character is taken from every pool
	// first so the result always satisfies the requested mix.
	var pools [][]rune
	addPool := func(enabled bool, chars string) {
		if !enabled {
			return
		}
		set := []rune(chars)
		if req.NoAmbiguous {
			set = without(set, ambiguousChars)
		}
		if len(set) > 0 {
			pools = append(pools, set)
		}
	}
	addPool(req.Lower, lowerChars)
	addPool(req.Upper, upperChars)
	addPool(req.Digits, digitChars)
	addPool(req.Symbols, symbolChars)

	if len(pools) == 0 {
		writeErr(w, http.StatusBadRequest, "pick at least one kind of character")
		return
	}
	if req.Length < len(pools) {
		writeErr(w, http.StatusBadRequest, "length is too short for that many character types")
		return
	}

	var everything []rune
	for _, p := range pools {
		everything = append(everything, p...)
	}

	results := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		out := make([]rune, 0, req.Length)
		for _, p := range pools {
			c, err := pick(p)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			out = append(out, c)
		}
		for len(out) < req.Length {
			c, err := pick(everything)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			out = append(out, c)
		}
		if err := shuffle(out); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		results = append(results, string(out))
	}

	// Entropy of the generating process: length x log2(alphabet size).
	bits := float64(req.Length) * math.Log2(float64(len(everything)))
	writeJSON(w, http.StatusOK, map[string]any{
		"passwords": results,
		"alphabet":  len(everything),
		"bits":      math.Round(bits*10) / 10,
		"strength":  strengthLabel(bits),
	})
}

func without(set []rune, exclude string) []rune {
	out := make([]rune, 0, len(set))
	for _, c := range set {
		if !strings.ContainsRune(exclude, c) {
			out = append(out, c)
		}
	}
	return out
}

func strengthLabel(bits float64) string {
	switch {
	case bits < 40:
		return "weak"
	case bits < 64:
		return "fair"
	case bits < 90:
		return "strong"
	case bits < 128:
		return "very strong"
	default:
		return "overkill"
	}
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

type usernameRequest struct {
	Base  string `json:"base"`
	Style string `json:"style"`
	Count int    `json:"count"`
}

var (
	usernameAdjectives = []string{
		"swift", "quiet", "bright", "hollow", "iron", "lunar", "amber", "rapid",
		"solar", "frost", "velvet", "crimson", "silent", "golden", "neon", "stone",
	}
	usernameNouns = []string{
		"falcon", "cipher", "harbor", "lantern", "ember", "ridge", "signal", "atlas",
		"vector", "willow", "cobalt", "circuit", "meadow", "pilot", "anchor", "comet",
	}
)

func (a *app) handleUsername(w http.ResponseWriter, r *http.Request) {
	req := usernameRequest{Style: "clean", Count: 8}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Count < 1 || req.Count > 50 {
		writeErr(w, http.StatusBadRequest, "count must be between 1 and 50")
		return
	}

	base := nonAlphanumeric.ReplaceAllString(strings.ToLower(strings.TrimSpace(req.Base)), "")
	if len(base) > 20 {
		base = base[:20]
	}

	results := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		name, err := makeUsername(base, req.Style)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		results = append(results, name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"usernames": results})
}

func makeUsername(base, style string) (string, error) {
	randomInt := func(max int64) (int64, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(max))
		if err != nil {
			return 0, fmt.Errorf("no secure randomness available: %w", err)
		}
		return n.Int64(), nil
	}
	word := func(list []string) (string, error) {
		n, err := randomInt(int64(len(list)))
		if err != nil {
			return "", err
		}
		return list[n], nil
	}

	switch style {
	case "words":
		adjective, err := word(usernameAdjectives)
		if err != nil {
			return "", err
		}
		noun, err := word(usernameNouns)
		if err != nil {
			return "", err
		}
		n, err := randomInt(100)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%s%02d", adjective, noun, n), nil

	case "dotted":
		if base == "" {
			base = "user"
		}
		noun, err := word(usernameNouns)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s", base, noun), nil

	case "numbered":
		if base == "" {
			base = "user"
		}
		n, err := randomInt(90000000)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%d", base, n+10000000), nil

	default: // "clean"
		if base == "" {
			adjective, err := word(usernameAdjectives)
			if err != nil {
				return "", err
			}
			base = adjective
		}
		noun, err := word(usernameNouns)
		if err != nil {
			return "", err
		}
		n, err := randomInt(1000)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s_%s%03d", base, noun, n), nil
	}
}

func (a *app) handleHash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	data := []byte(req.Text)
	md5sum := md5.Sum(data)
	sha1sum := sha1.Sum(data)
	sha256sum := sha256.Sum256(data)
	sha512sum := sha512.Sum512(data)
	writeJSON(w, http.StatusOK, map[string]any{
		"bytes":  len(data),
		"md5":    hex.EncodeToString(md5sum[:]),
		"sha1":   hex.EncodeToString(sha1sum[:]),
		"sha256": hex.EncodeToString(sha256sum[:]),
		"sha512": hex.EncodeToString(sha512sum[:]),
	})
}

func (a *app) handleTokens(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bytes int `json:"bytes"`
		Count int `json:"count"`
	}
	req.Bytes, req.Count = 32, 5
	if !readJSON(w, r, &req) {
		return
	}
	if req.Bytes < 4 || req.Bytes > 256 {
		writeErr(w, http.StatusBadRequest, "size must be between 4 and 256 bytes")
		return
	}
	if req.Count < 1 || req.Count > 25 {
		writeErr(w, http.StatusBadRequest, "count must be between 1 and 25")
		return
	}

	hexes := make([]string, 0, req.Count)
	b64s := make([]string, 0, req.Count)
	uuids := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		buf := make([]byte, req.Bytes)
		if _, err := rand.Read(buf); err != nil {
			writeErr(w, http.StatusInternalServerError, "no secure randomness available")
			return
		}
		hexes = append(hexes, hex.EncodeToString(buf))
		b64s = append(b64s, base64.RawURLEncoding.EncodeToString(buf))

		u, err := uuidV4()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		uuids = append(uuids, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"hex": hexes, "base64": b64s, "uuid": uuids})
}

// uuidV4 builds a random (version 4) UUID.
func uuidV4() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("no secure randomness available: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}

func (a *app) handleText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
		Op   string `json:"op"`
	}
	if !readJSON(w, r, &req) {
		return
	}

	var out string
	var err error
	switch req.Op {
	case "upper":
		out = strings.ToUpper(req.Text)
	case "lower":
		out = strings.ToLower(req.Text)
	case "title":
		out = titleCase(req.Text)
	case "trim":
		out = strings.TrimSpace(req.Text)
	case "reverse":
		out = reverseString(req.Text)
	case "slug":
		out = nonAlphanumeric.ReplaceAllString(strings.ToLower(strings.TrimSpace(req.Text)), "-")
		out = strings.Trim(out, "-")
	case "b64encode":
		out = base64.StdEncoding.EncodeToString([]byte(req.Text))
	case "b64decode":
		decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Text))
		if decodeErr != nil {
			err = fmt.Errorf("that isn't valid base64")
		} else {
			out = string(decoded)
		}
	case "urlencode":
		out = url.QueryEscape(req.Text)
	case "urldecode":
		decoded, decodeErr := url.QueryUnescape(req.Text)
		if decodeErr != nil {
			err = fmt.Errorf("that isn't valid URL encoding")
		} else {
			out = decoded
		}
	case "sortlines":
		lines := splitLines(req.Text)
		sort.Strings(lines)
		out = strings.Join(lines, "\n")
	case "dedupe":
		out = strings.Join(dedupeLines(splitLines(req.Text)), "\n")
	case "shufflelines":
		lines := splitLines(req.Text)
		runeIdx := make([]rune, len(lines))
		for i := range runeIdx {
			runeIdx[i] = rune(i)
		}
		if shuffleErr := shuffle(runeIdx); shuffleErr != nil {
			err = shuffleErr
		} else {
			shuffled := make([]string, len(lines))
			for i, j := range runeIdx {
				shuffled[i] = lines[int(j)]
			}
			out = strings.Join(shuffled, "\n")
		}
	default:
		err = fmt.Errorf("unknown operation %q", req.Op)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": out,
		"stats": map[string]int{
			"characters": len([]rune(req.Text)),
			"words":      len(strings.Fields(req.Text)),
			"lines":      len(splitLines(req.Text)),
			"bytes":      len(req.Text),
		},
	})
}

func titleCase(s string) string {
	words := strings.Split(strings.ToLower(s), " ")
	for i, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		words[i] = strings.ToUpper(string(runes[0])) + string(runes[1:])
	}
	return strings.Join(words, " ")
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func dedupeLines(lines []string) []string {
	seen := make(map[string]bool, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out
}

func (a *app) handleSysinfo(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	username := "unknown"
	if u := os.Getenv("USERNAME"); u != "" {
		username = u
	} else if u := os.Getenv("USER"); u != "" {
		username = u
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"hostname":   hostname,
		"username":   username,
		"goVersion":  runtime.Version(),
		"appVersion": version,
		"uptime":     formatDuration(time.Since(a.startedAt)),
		"memoryMB":   math.Round(float64(mem.Sys)/(1024*1024)*10) / 10,
		"addresses":  localAddresses(),
		"localTime":  time.Now().Format("Mon 2 Jan 2006, 15:04:05 MST"),
		"dataDir":    dataDirOrEmpty(),
	})
}

func dataDirOrEmpty() string {
	dir, err := dataDir()
	if err != nil {
		return ""
	}
	return dir
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// localAddresses lists this machine's non-loopback IP addresses.
func localAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, fmt.Sprintf("%s (%s)", ipNet.IP.String(), iface.Name))
		}
	}
	sort.Strings(out)
	return out
}
