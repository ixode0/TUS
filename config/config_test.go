package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func validJSON() string {
	return `{"telegram":{"phone_number":"+1234567890","api_id":123,"api_hash":"abc"},"claim_to":"channel","sleep_between_check":100,"usernames":["durovv"]}`
}

func TestLoadValid(t *testing.T) {
	p := writeCfg(t, validJSON())
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.ClaimTo != "channel" || len(cfg.Usernames) != 1 {
		t.Fatalf("bad cfg: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := []struct {
		name string
		mut  string
	}{
		{"bad phone", `{"telegram":{"phone_number":"123","api_id":1,"api_hash":"a"},"claim_to":"channel","sleep_between_check":100,"usernames":["durovv"]}`},
		{"zero api_id", `{"telegram":{"phone_number":"+12345","api_id":0,"api_hash":"a"},"claim_to":"channel","sleep_between_check":100,"usernames":["durovv"]}`},
		{"empty hash", `{"telegram":{"phone_number":"+12345","api_id":1,"api_hash":""},"claim_to":"channel","sleep_between_check":100,"usernames":["durovv"]}`},
		{"bad claim_to", `{"telegram":{"phone_number":"+12345","api_id":1,"api_hash":"a"},"claim_to":"x","sleep_between_check":100,"usernames":["durovv"]}`},
		{"no usernames", `{"telegram":{"phone_number":"+12345","api_id":1,"api_hash":"a"},"claim_to":"channel","sleep_between_check":100,"usernames":[]}`},
		{"bad username", `{"telegram":{"phone_number":"+12345","api_id":1,"api_hash":"a"},"claim_to":"channel","sleep_between_check":100,"usernames":["ab"]}`},
		{"bad sleep", `{"telegram":{"phone_number":"+12345","api_id":1,"api_hash":"a"},"claim_to":"channel","sleep_between_check":0,"usernames":["durovv"]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Load(writeCfg(t, c.mut)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
