package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Safe defaults + docs:
//
//	sleep_between_check: pause between monitor rounds, ms.
//	  Default 100 when missing/0. Values <50 risk FloodWait (only a warning,
//	  run() logs it). See README "Конфигурация".
//	CLAIM_MAX_ATTEMPTS (env, sniper): bounds transient retry loop.
//	  Default 0 = unlimited (never silently drop a snipe; stop only on
//	  permanent error or ctx cancel). Set e.g. CLAIM_MAX_ATTEMPTS=200 to cap.
const (
	DefaultSleepMS = 100
	MinSafeSleepMS = 50
)

var (
	config     *Configuration
	loadOnce   sync.Once
	loadErr    error
	usernameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,31}$`)
	phoneRe    = regexp.MustCompile(`^\+\d{4,15}$`)
)

type Configuration struct {
	Telegram struct {
		PhoneNumber string `json:"phone_number"`
		APIID       int    `json:"api_id"`
		APIHash     string `json:"api_hash"`
	} `json:"telegram"`
	ClaimTo          string   `json:"claim_to"`
	CheckSleepTimeMS int      `json:"sleep_between_check"`
	Usernames        []string `json:"usernames"`
}

// Load reads, parses and validates a config file.
func Load(path string) (*Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config not found %q: %w\n\n%s", path, err, startupHelp(path))
	}
	defer file.Close()

	cfg := &Configuration{}
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return cfg, nil
}

// Normalize trims/normalizes user input so small formatting slips don't
// become hard errors: phones lose spaces/dashes/parens, usernames lose
// leading @ and surrounding spaces, claim_to is lowercased.
func (c *Configuration) Normalize() {
	c.Telegram.PhoneNumber = NormalizePhone(c.Telegram.PhoneNumber)
	c.Telegram.APIHash = strings.TrimSpace(c.Telegram.APIHash)
	c.ClaimTo = strings.ToLower(strings.TrimSpace(c.ClaimTo))
	for i, u := range c.Usernames {
		c.Usernames[i] = NormalizeUsername(u)
	}
	if c.CheckSleepTimeMS == 0 {
		c.CheckSleepTimeMS = DefaultSleepMS
	}
}

// NormalizePhone removes spaces, dashes and parens: "+7 999-123 45-67"
// -> "+79991234567". Leading/trailing space is trimmed first.
func NormalizePhone(s string) string {
	s = strings.TrimSpace(s)
	// strip inner spaces, dashes, parens
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	return s
}

// NormalizeUsername strips surrounding spaces and leading @, lowercases:
// " @Durov " -> "durov". Telegram usernames are case-insensitive.
func NormalizeUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSpace(s)
	return strings.ToLower(s)
}

// Validate checks all fields and returns a human-readable error
// (no raw regex dumps).
func (c *Configuration) Validate() error {
	ph := c.Telegram.PhoneNumber
	switch {
	case ph == "":
		return fmt.Errorf("telegram.phone_number пуст: напиши телефон только с +, убери пробелы и тире, например +79251234567 (8 в начале замени на +7)")
	case strings.HasPrefix(ph, "8") && !strings.HasPrefix(ph, "+"):
		return fmt.Errorf("telegram.phone_number %q: начинается с 8 — замени 8 на +7, например +79251234567 (телефон только с +, убери пробелы и тире)", ph)
	case !strings.HasPrefix(ph, "+"):
		return fmt.Errorf("telegram.phone_number %q: телефон только с +, убери пробелы и тире, например +79251234567 (8 в начале замени на +7)", ph)
	case !phoneRe.MatchString(ph):
		return fmt.Errorf("telegram.phone_number %q: телефон только с +, убери пробелы, тире и скобки — после + только цифры, 4-15 цифр, например +79251234567", ph)
	}
	if c.Telegram.APIID == 0 {
		return fmt.Errorf("telegram.api_id пуст: возьми на https://my.telegram.org → API development tools и впиши в config.json")
	}
	if strings.TrimSpace(c.Telegram.APIHash) == "" {
		return fmt.Errorf("telegram.api_hash пуст: возьми на https://my.telegram.org → API development tools и впиши в config.json")
	}
	if c.ClaimTo != "user" && c.ClaimTo != "channel" {
		return fmt.Errorf("claim_to %q: напиши маленькими буквами channel или user (без @, без больших букв)", c.ClaimTo)
	}
	if len(c.Usernames) == 0 {
		return fmt.Errorf("usernames пуст: добавь хотя бы 1 юзернейм без @, например [\"durov\"]")
	}
	for _, u := range c.Usernames {
		if !usernameRe.MatchString(u) {
			return fmt.Errorf("юзернейм %q: 4-32 символа без @, буквы/цифры/_, начинается с буквы (убери @ в начале, например @durov → durov)", u)
		}
	}
	if c.CheckSleepTimeMS < 0 {
		return fmt.Errorf("sleep_between_check %d: должен быть >= 0 мс (0 = по умолчанию %d мс; меньше %d мс — риск FloodWait)", c.CheckSleepTimeMS, DefaultSleepMS, MinSafeSleepMS)
	}
	return nil
}

// startupHelp is the single start screen shown when config is missing:
// where the config lives, and where to get api_id/api_hash.
func startupHelp(triedPath string) string {
	home, _ := os.UserHomeDir()
	xdg := filepath.Join(home, ".config", "tus", "config.json")
	return fmt.Sprintf(`=== TUS: нет конфига — с чего начать ===
1. Конфиг ищется тут (первый найденный побеждает):
   - рядом с запуском: ./config.json
   - рядом с бинарём:  <папка_бинаря>/config.json
   - общий путь:       %s
   - свой путь:        CONFIG_PATH=/path/to/config.json ./sniper
   Пробовал: %s
2. Создай: cp config.example.json config.json (или положи в ~/.config/tus/config.json)
3. api_id и api_hash возьми на https://my.telegram.org → API development tools
   (залогинься номером телефона, создай app, впиши id+hash в config.json).
4. Телефон пиши с + без пробелов/тире (8 в начале → +7). Юзернеймы — без @.
5. Запуск: ./sniper (первый раз попросит код из Telegram).`, xdg, triedPath)
}

// ConfigSearchPaths returns searched locations in order (for docs/tests).
func ConfigSearchPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{"config.json"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "config.json"))
	}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".config", "tus", "config.json"))
	}
	return paths
}

// GetConfig loads the config once (thread-safe) and returns a COPY.
// It terminates the process with a clear message if loading fails,
// preserving the old no-error signature for existing callers.
// Prefer Load() in new code and tests.
func GetConfig() *Configuration {
	loadOnce.Do(func() {
		path := getConfigFilePath()
		var cfg *Configuration
		cfg, loadErr = Load(path)
		if loadErr != nil {
			//nolint:revive // intentional fatal: old API has no error return
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", loadErr)
			os.Exit(1)
		}
		config = cfg
	})
	// Return a deep copy so callers can't mutate global state (data race fix).
	cp := *config
	cp.Usernames = append([]string(nil), config.Usernames...)
	return &cp
}

func getConfigFilePath() string {
	// Allow override via env
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}
	// Try working dir first
	if _, err := os.Stat("config.json"); err == nil {
		return "config.json"
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return "config.json"
	}

	if filepath.Base(currentDir) == "app" {
		candidate := filepath.Join(currentDir, "..", "..", "config.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return candidate
	}

	executablePath, err := os.Executable()
	if err != nil {
		return filepath.Join(currentDir, "config.json")
	}

	executableDir := filepath.Dir(executablePath)
	candidate := filepath.Join(executableDir, "config.json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// shared user config: ~/.config/tus/config.json (used by install.sh launcher)
	if home, herr := os.UserHomeDir(); herr == nil && home != "" {
		xdg := filepath.Join(home, ".config", "tus", "config.json")
		if _, err := os.Stat(xdg); err == nil {
			return xdg
		}
	}
	// fallback to cwd
	return filepath.Join(currentDir, "config.json")
}

// SessionPath returns a stable absolute path for the MTProto session file,
// stored next to the resolved config file (or via SESSION_PATH override).
// This fixes the old CWD-dependent behaviour where running from different
// directories silently created different sessions.
func SessionPath() string {
	if env := os.Getenv("SESSION_PATH"); env != "" {
		return env
	}
	cfgPath := getConfigFilePath()
	if abs, err := filepath.Abs(cfgPath); err == nil {
		return filepath.Join(filepath.Dir(abs), "session_DO_NOT_SHARE.json")
	}
	return "session_DO_NOT_SHARE.json"
}
