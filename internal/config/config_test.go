package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedenceAndStrictJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"port":3307,"rows":5}`), 0600)
	c, err := Load(p, func(s string) (string, bool) { return "3308", s == "MYBENCHX_PORT" }, map[string]string{"port": "3309"})
	if err != nil || c.Port != 3309 || c.Rows != 5 {
		t.Fatalf("precedence: %+v %v", c, err)
	}
	for _, body := range []string{`{"password":"private"}`, `{"port":1,"port":2}`, `{} {}`, `null`, `{"rows":-1}`, `{"batch":0}`, `{"workload":"typo"}`, `{"table_prefix":"x;DROP"}`} {
		os.WriteFile(p, []byte(body), 0600)
		if _, err := Load(p, func(string) (string, bool) { return "", false }, nil); err == nil {
			t.Errorf("accepted invalid input %s", body)
		}
	}
}

func TestSecretPermissionsAndReference(t *testing.T) {
	p := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(p, []byte("test-only\n"), 0600)
	if s, err := Secret("", p); err != nil || s != "test-only" {
		t.Fatal("private file failed", err)
	}
	os.Chmod(p, 0644)
	if _, err := Secret("", p); err == nil {
		t.Fatal("accepted world-readable credential")
	}
	t.Setenv("MYBENCHX_TEST_PASSWORD", "env-test")
	if s, err := Secret("MYBENCHX_TEST_PASSWORD", ""); err != nil || s != "env-test" {
		t.Fatal("environment credential failed")
	}
}
