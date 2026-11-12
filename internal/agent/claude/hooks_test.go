package claude

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const existing = `{
  "model": "opus",
  "hooks": {
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "say done && echo <ok>" },
          { "type": "command", "command": "\"/old/botler\" hook claude" }
        ]
      }
    ],
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "guard" }] }
    ]
  },
  "theme": "dark"
}
`

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(home, ".claude", "settings.json")
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestInstallKeepsOtherSettings(t *testing.T) {
	p := writeSettings(t, existing)
	exe := "/Applications/Botler.app/Contents/MacOS/botler"

	st, err := Status(exe)
	if err != nil || st.Installed || !st.Outdated {
		t.Fatalf("Status before = %+v, %v; want outdated", st, err)
	}
	if err := Install(exe); err != nil {
		t.Fatal(err)
	}
	st, err = Status(exe)
	if err != nil || !st.Installed || st.Outdated {
		t.Fatalf("Status after = %+v, %v; want installed", st, err)
	}

	data, _ := os.ReadFile(p)
	got := string(data)
	for _, want := range []string{`"say done && echo <ok>"`, `"guard"`, `"theme": "dark"`, `"StopFailure"`, `"Notification"`} {
		if !strings.Contains(got, want) {
			t.Errorf("settings missing %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/old/botler") {
		t.Errorf("old hook still present:\n%s", got)
	}
	if strings.Index(got, `"model"`) > strings.Index(got, `"hooks"`) || strings.Index(got, `"hooks"`) > strings.Index(got, `"theme"`) {
		t.Errorf("key order changed:\n%s", got)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}

	if err := Install(exe); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p)
	if n := strings.Count(string(data), "botler\\\" hook claude"); n != 3 {
		t.Errorf("after reinstall %d hooks, want 3:\n%s", n, data)
	}

	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p)
	got = string(data)
	if strings.Contains(got, "botler") || strings.Contains(got, "StopFailure") || strings.Contains(got, "Notification") {
		t.Errorf("uninstall left hooks behind:\n%s", got)
	}
	if !strings.Contains(got, `"say done && echo <ok>"`) || !strings.Contains(got, `"guard"`) {
		t.Errorf("uninstall removed user hooks:\n%s", got)
	}
}

func TestInstallCreatesSettings(t *testing.T) {
	p := writeSettings(t, "")
	if err := Install("/bin/botler"); err != nil {
		t.Fatal(err)
	}
	if st, err := Status("/bin/botler"); err != nil || !st.Installed {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if strings.TrimSpace(string(data)) != "{}" {
		t.Errorf("settings after uninstall = %q, want {}", data)
	}
}

func TestInvalidSettingsUntouched(t *testing.T) {
	p := writeSettings(t, "{ not json")
	if err := Install("/bin/botler"); err == nil {
		t.Fatal("Install on invalid JSON succeeded")
	}
	if data, _ := os.ReadFile(p); string(data) != "{ not json" {
		t.Errorf("file changed: %q", data)
	}
}

func TestHookCommandSkipsMissingBinary(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", HookCommand("/nonexistent/Botler.app/Contents/MacOS/botler"))
	if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("missing binary: err %v, output %q", err, out)
	}
	if !isOurs(HookCommand("/Applications/Botler.app/Contents/MacOS/botler")) {
		t.Fatal("isOurs does not recognise HookCommand")
	}
}
