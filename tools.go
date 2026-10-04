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

// PasswordRequest is what the password panel sends.
type PasswordRequest struct {
	Length      int  `json:"length"`
	Count       int  `json:"count"`
	Lower       bool `json:"lower"`
	Upper       bool `json:"upper"`
	Digits      bool `json:"digits"`
	Symbols     bool `json:"symbols"`
	NoAmbiguous bool `json:"noAmbiguous"`
}

// PasswordResult is the generated set plus how strong it is.
type PasswordResult struct {
	Passwords []string `json:"passwords"`
	Alphabet  int      `json:"alphabet"`
	Bits      float64  `json:"bits"`
	Strength  string   `json:"strength"`
}

// GeneratePasswords returns random passwords matching the requested mix.
func (a *App) GeneratePasswords(req PasswordRequest) (PasswordResult, error) {
	if req.Length < 4 || req.Length > 256 {
		return PasswordResult{}, fmt.Errorf("length must be between 4 and 256")
	}
	if req.Count < 1 || req.Count > 50 {
		return PasswordResult{}, fmt.Errorf("count must be between 1 and 50")
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
		return PasswordResult{}, fmt.Errorf("pick at least one kind of character")
	}
	if req.Length < len(pools) {
		return PasswordResult{}, fmt.Errorf("length is too short for that many character types")
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
				return PasswordResult{}, err
			}
			out = append(out, c)
		}
		for len(out) < req.Length {
			c, err := pick(everything)
			if err != nil {
				return PasswordResult{}, err
			}
			out = append(out, c)
		}
		if err := shuffle(out); err != nil {
			return PasswordResult{}, err
		}
		results = append(results, string(out))
	}

	// Entropy of the generating process: length x log2(alphabet size).
	bits := float64(req.Length) * math.Log2(float64(len(everything)))
	return PasswordResult{
		Passwords: results,
		Alphabet:  len(everything),
		Bits:      math.Round(bits*10) / 10,
		Strength:  strengthLabel(bits),
	}, nil
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

// UsernameRequest is what the username panel sends.
type UsernameRequest struct {
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

// GenerateUsernames returns name suggestions in the requested style.
func (a *App) GenerateUsernames(req UsernameRequest) ([]string, error) {
	if req.Count < 1 || req.Count > 50 {
		return nil, fmt.Errorf("count must be between 1 and 50")
	}

	base := nonAlphanumeric.ReplaceAllString(strings.ToLower(strings.TrimSpace(req.Base)), "")
	if len(base) > 20 {
		base = base[:20]
	}

	results := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		name, err := makeUsername(base, req.Style)
		if err != nil {
			return nil, err
		}
		results = append(results, name)
	}
	return results, nil
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

// HashResult holds the digests of some text.
type HashResult struct {
	Bytes  int    `json:"bytes"`
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
	SHA512 string `json:"sha512"`
}

// HashText returns the common digests of the given text.
func (a *App) HashText(text string) HashResult {
	data := []byte(text)
	md5sum := md5.Sum(data)
	sha1sum := sha1.Sum(data)
	sha256sum := sha256.Sum256(data)
	sha512sum := sha512.Sum512(data)
	return HashResult{
		Bytes:  len(data),
		MD5:    hex.EncodeToString(md5sum[:]),
		SHA1:   hex.EncodeToString(sha1sum[:]),
		SHA256: hex.EncodeToString(sha256sum[:]),
		SHA512: hex.EncodeToString(sha512sum[:]),
	}
}

// TokenRequest is what the keys panel sends.
type TokenRequest struct {
	Bytes int `json:"bytes"`
	Count int `json:"count"`
}

// TokenResult holds the same random values in three encodings.
type TokenResult struct {
	Hex    []string `json:"hex"`
	Base64 []string `json:"base64"`
	UUID   []string `json:"uuid"`
}

// GenerateTokens returns random values as hex, base64url and UUIDs.
func (a *App) GenerateTokens(req TokenRequest) (TokenResult, error) {
	if req.Bytes < 4 || req.Bytes > 256 {
		return TokenResult{}, fmt.Errorf("size must be between 4 and 256 bytes")
	}
	if req.Count < 1 || req.Count > 25 {
		return TokenResult{}, fmt.Errorf("count must be between 1 and 25")
	}

	out := TokenResult{}
	for i := 0; i < req.Count; i++ {
		buf := make([]byte, req.Bytes)
		if _, err := rand.Read(buf); err != nil {
			return TokenResult{}, fmt.Errorf("no secure randomness available: %w", err)
		}
		out.Hex = append(out.Hex, hex.EncodeToString(buf))
		out.Base64 = append(out.Base64, base64.RawURLEncoding.EncodeToString(buf))

		u, err := uuidV4()
		if err != nil {
			return TokenResult{}, err
		}
		out.UUID = append(out.UUID, u)
	}
	return out, nil
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

// TextStats counts what was fed in.
type TextStats struct {
	Characters int `json:"characters"`
	Words      int `json:"words"`
	Lines      int `json:"lines"`
	Bytes      int `json:"bytes"`
}

// TextResult is a converted string plus the stats of the input.
type TextResult struct {
	Result string    `json:"result"`
	Stats  TextStats `json:"stats"`
}

// TransformText applies one named conversion to the given text.
func (a *App) TransformText(text, op string) (TextResult, error) {
	var out string
	var err error

	switch op {
	case "upper":
		out = strings.ToUpper(text)
	case "lower":
		out = strings.ToLower(text)
	case "title":
		out = titleCase(text)
	case "trim":
		out = strings.TrimSpace(text)
	case "reverse":
		out = reverseString(text)
	case "slug":
		out = nonAlphanumeric.ReplaceAllString(strings.ToLower(strings.TrimSpace(text)), "-")
		out = strings.Trim(out, "-")
	case "b64encode":
		out = base64.StdEncoding.EncodeToString([]byte(text))
	case "b64decode":
		decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if decodeErr != nil {
			err = fmt.Errorf("that isn't valid base64")
		} else {
			out = string(decoded)
		}
	case "urlencode":
		out = url.QueryEscape(text)
	case "urldecode":
		decoded, decodeErr := url.QueryUnescape(text)
		if decodeErr != nil {
			err = fmt.Errorf("that isn't valid URL encoding")
		} else {
			out = decoded
		}
	case "sortlines":
		lines := splitLines(text)
		sort.Strings(lines)
		out = strings.Join(lines, "\n")
	case "dedupe":
		out = strings.Join(dedupeLines(splitLines(text)), "\n")
	case "shufflelines":
		lines := splitLines(text)
		order := make([]rune, len(lines))
		for i := range order {
			order[i] = rune(i)
		}
		if shuffleErr := shuffle(order); shuffleErr != nil {
			err = shuffleErr
		} else {
			shuffled := make([]string, len(lines))
			for i, j := range order {
				shuffled[i] = lines[int(j)]
			}
			out = strings.Join(shuffled, "\n")
		}
	default:
		err = fmt.Errorf("unknown operation %q", op)
	}
	if err != nil {
		return TextResult{}, err
	}

	return TextResult{
		Result: out,
		Stats: TextStats{
			Characters: len([]rune(text)),
			Words:      len(strings.Fields(text)),
			Lines:      len(splitLines(text)),
			Bytes:      len(text),
		},
	}, nil
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

// SysInfo describes the machine the app is running on.
type SysInfo struct {
	OS         string   `json:"os"`
	Arch       string   `json:"arch"`
	CPUs       int      `json:"cpus"`
	Hostname   string   `json:"hostname"`
	Username   string   `json:"username"`
	GoVersion  string   `json:"goVersion"`
	AppVersion string   `json:"appVersion"`
	Uptime     string   `json:"uptime"`
	MemoryMB   float64  `json:"memoryMB"`
	Addresses  []string `json:"addresses"`
	LocalTime  string   `json:"localTime"`
	DataDir    string   `json:"dataDir"`
}

// SystemInfo reports what this machine looks like.
func (a *App) SystemInfo() SysInfo {
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

	return SysInfo{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUs:       runtime.NumCPU(),
		Hostname:   hostname,
		Username:   username,
		GoVersion:  runtime.Version(),
		AppVersion: version,
		Uptime:     formatDuration(time.Since(a.startedAt)),
		MemoryMB:   math.Round(float64(mem.Sys)/(1024*1024)*10) / 10,
		Addresses:  localAddresses(),
		LocalTime:  time.Now().Format("Mon 2 Jan 2006, 15:04:05 MST"),
		DataDir:    dataDirOrEmpty(),
	}
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
