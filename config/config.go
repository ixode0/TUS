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

var (
	config     *Configuration
	loadOnce   sync.Once
	loadErr    error
	usernameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,31}$`)
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
		return nil, fmt.Errorf("open config %q: %w (copy config.example.json to config.json or set CONFIG_PATH)", path, err)
	}
	defer file.Close()

	cfg := &Configuration{}
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return cfg, nil
}

// Validate checks all fields and returns a descriptive error.
func (c *Configuration) Validate() error {
	if !strings.HasPrefix(c.Telegram.PhoneNumber, "+") || len(c.Telegram.PhoneNumber) < 5 {
		return fmt.Errorf("telegram.phone_number must be in international format (e.g. +1234567890)")
	}
	if c.Telegram.APIID == 0 {
		return fmt.Errorf("telegram.api_id is required (get it at https://my.telegram.org)")
	}
	if strings.TrimSpace(c.Telegram.APIHash) == "" {
		return fmt.Errorf("telegram.api_hash is required (get it at https://my.telegram.org)")
	}
	if c.ClaimTo != "user" && c.ClaimTo != "channel" {
		return fmt.Errorf("claim_to must be either 'user' or 'channel', got %q", c.ClaimTo)
	}
	if len(c.Usernames) == 0 {
		return fmt.Errorf("usernames must contain at least 1 username")
	}
	for _, u := range c.Usernames {
		if !usernameRe.MatchString(u) {
			return fmt.Errorf("invalid username %q: must match %s (letters/digits/underscore, 4-32 chars, start with letter)", u, usernameRe.String())
		}
	}
	if c.CheckSleepTimeMS <= 0 {
		return fmt.Errorf("sleep_between_check must be > 0 ms")
	}
	return nil
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
