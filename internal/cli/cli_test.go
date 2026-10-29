package cli

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachineProtocolAndWriteAuthorization(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"describe", "--output", "json"}, 0},
		{[]string{"plan", "--rows", "12", "--output", "json"}, 0},
		{[]string{"config", "show", "--port", "3311", "--output", "json"}, 0},
		{[]string{"random", "--output", "json"}, 2},
		{[]string{"run", "--read-workers", "0", "--output", "json"}, 2},
		{[]string{"run", "--write-threads", "1", "--output", "json"}, 2},
		{[]string{"run", "--insert-workers", "1", "--output", "json"}, 2},
		{[]string{"cleanup", "--output", "json"}, 2},
		{[]string{"prepare", "--output", "json"}, 2},
		{[]string{"run", "--batch", "bad", "--output", "ndjson"}, 2},
		{[]string{"version", "--unknown", "must-not-echo", "--output", "json"}, 2},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := Execute(t.Context(), tc.args, &out, &diagnostic)
			if code != tc.code {
				t.Fatalf("exit=%d output=%s", code, out.String())
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("not one JSON object: %s %v", out.String(), err)
			}
			if result["schema_version"] != "1" {
				t.Fatal(result)
			}
			if strings.Contains(out.String(), "must-not-echo") {
				t.Fatal("echoed untrusted argument")
			}
		})
	}
}

func TestConfigNeverEchoesUnknownSecret(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(p, []byte(`{"password":"test-secret-do-not-echo"}`), 0600)
	var out bytes.Buffer
	code := Execute(t.Context(), []string{"config", "validate", "--config", p, "--output", "json"}, &out, &out)
	if code != 2 || strings.Contains(out.String(), "test-secret-do-not-echo") {
		t.Fatal("invalid config leaked secret")
	}
}
