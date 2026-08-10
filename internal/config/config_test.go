package config

import (
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		"MATRIX_HOMESERVER": "https://matrix.example.com",
		"MATRIX_USER":       "@logs:matrix.example.com",
		"MATRIX_TOKEN":      "syt_secret",
		"MATRIX_ROOM":       "!room:matrix.example.com",
	}
}

func TestLoad_OK(t *testing.T) {
	setEnv(t, baseEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Homeserver != "https://matrix.example.com" {
		t.Fatalf("homeserver = %q", cfg.Homeserver)
	}
	if cfg.Rate.Milliseconds() != 300 {
		t.Fatalf("rate = %v, want 300ms", cfg.Rate)
	}
	if cfg.MaxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", cfg.MaxRetries)
	}
}

func TestLoad_MissingAll(t *testing.T) {
	for _, k := range []string{
		"MATRIX_HOMESERVER", "MATRIX_USER", "MATRIX_TOKEN", "MATRIX_ROOM",
	} {
		t.Setenv(k, "")
	}
	_, err := Load()
	if err == nil {
		t.Fatal("expected error on missing env")
	}
	for _, want := range []string{"MATRIX_HOMESERVER", "MATRIX_USER", "MATRIX_TOKEN", "MATRIX_ROOM"} {
		if !contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestLoad_HomeserverScheme(t *testing.T) {
	for _, hs := range []string{
		"matrix.example.com",
		"ftp://matrix.example.com",
		"//matrix.example.com",
	} {
		t.Run(hs, func(t *testing.T) {
			e := baseEnv()
			e["MATRIX_HOMESERVER"] = hs
			setEnv(t, e)
			if _, err := Load(); err == nil {
				t.Fatalf("expected scheme error for %q", hs)
			}
		})
	}
}

func TestLoad_RoomMustBeID(t *testing.T) {
	e := baseEnv()
	e["MATRIX_ROOM"] = "#logs:matrix.example.com"
	setEnv(t, e)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for alias room")
	}
}

func TestLoad_BadInts(t *testing.T) {
	e := baseEnv()
	e["MATRIX_RATE_MS"] = "abc"
	setEnv(t, e)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad MATRIX_RATE_MS")
	}
}

func TestLoad_BadDryRun(t *testing.T) {
	e := baseEnv()
	e["MATRIX_DRY_RUN"] = "yes please"
	setEnv(t, e)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad MATRIX_DRY_RUN")
	}
}

func TestLoad_NegativeIntsRejected(t *testing.T) {
	e := baseEnv()
	e["MATRIX_MAX_RETRIES"] = "-1"
	setEnv(t, e)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative MATRIX_MAX_RETRIES")
	}
}

func TestLoad_Debug(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"default off", nil, false},
		{"MATRIX_DEBUG=1", map[string]string{"MATRIX_DEBUG": "1"}, true},
		{"MATRIX_DEBUG=false", map[string]string{"MATRIX_DEBUG": "false"}, false},
		{"DEBUG=1", map[string]string{"DEBUG": "1"}, true},
		{"DEBUG=true", map[string]string{"DEBUG": "true"}, true},
		{"both set, either wins", map[string]string{"MATRIX_DEBUG": "0", "DEBUG": "1"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// clear both env vars first
			t.Setenv("MATRIX_DEBUG", "")
			t.Setenv("DEBUG", "")
			e := baseEnv()
			for k, v := range c.env {
				e[k] = v
			}
			setEnv(t, e)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Debug != c.want {
				t.Fatalf("Debug = %v, want %v", cfg.Debug, c.want)
			}
		})
	}
}

func TestLoad_BadDebug(t *testing.T) {
	e := baseEnv()
	e["MATRIX_DEBUG"] = "yes please"
	setEnv(t, e)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad MATRIX_DEBUG")
	}
}
