package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TheWolfNL/botler/internal/agent"
)

// HookEvents are the Claude Code hook events Botler listens to.
var HookEvents = []string{"Stop", "StopFailure", "Notification"}

const hookTimeout = 10

// SettingsPath returns ~/.claude/settings.json.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// HookCommand runs exe's hook handler and exits 0 once exe is uninstalled, so Claude Code reports no failing hook.
func HookCommand(exe string) string {
	q := strconv.Quote(exe)
	return "[ ! -x " + q + " ] || " + q + " hook " + Agent{}.ID()
}

// Status reports whether every hook event runs exe; Outdated means some point at another Botler binary.
func Status(exe string) (agent.HookStatus, error) {
	path, err := SettingsPath()
	if err != nil {
		return agent.HookStatus{}, err
	}
	st := agent.HookStatus{Settings: path}
	root, err := readSettings(path)
	if err != nil {
		return st, err
	}
	hooks, err := root.object("hooks")
	if err != nil {
		return st, err
	}
	want := HookCommand(exe)
	current := 0
	for _, ev := range HookEvents {
		cmds, err := commands(hooks, ev)
		if err != nil {
			return st, err
		}
		for _, c := range cmds {
			if c == want {
				current++
			} else if isOurs(c) {
				st.Outdated = true
			}
		}
	}
	st.Installed = current == len(HookEvents)
	return st, nil
}

// Install adds a hook running exe for every hook event, replacing hooks of other Botler binaries.
func Install(exe string) error {
	return edit(func(hooks *object) error {
		for _, ev := range HookEvents {
			groups, err := withoutOurs(hooks, ev)
			if err != nil {
				return err
			}
			group, err := marshal(newHookGroup(exe))
			if err != nil {
				return err
			}
			if err := hooks.setArray(ev, append(groups, group)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Uninstall removes every Botler hook, leaving other hooks untouched.
func Uninstall() error {
	return edit(func(hooks *object) error {
		for _, ev := range HookEvents {
			groups, err := withoutOurs(hooks, ev)
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				hooks.delete(ev)
				continue
			}
			if err := hooks.setArray(ev, groups); err != nil {
				return err
			}
		}
		return nil
	})
}

// isOurs recognises any Botler hook command, including ones from other installs of the app.
func isOurs(command string) bool {
	return strings.Contains(command, "botler") && strings.Contains(command, "\" hook")
}

type hookEntry struct {
	Command string `json:"command"`
}

type hookGroup struct {
	Hooks []json.RawMessage `json:"hooks"`
}

func commands(hooks object, event string) ([]string, error) {
	groups, err := hooks.array(event)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range groups {
		var hg hookGroup
		if err := json.Unmarshal(g, &hg); err != nil {
			return nil, fmt.Errorf("parse %s hooks: %w", event, err)
		}
		for _, h := range hg.Hooks {
			var e hookEntry
			if err := json.Unmarshal(h, &e); err == nil {
				out = append(out, e.Command)
			}
		}
	}
	return out, nil
}

// withoutOurs returns event's matcher groups with Botler hooks removed, dropping groups left empty.
func withoutOurs(hooks *object, event string) ([]json.RawMessage, error) {
	groups, err := hooks.array(event)
	if err != nil {
		return nil, err
	}
	var out []json.RawMessage
	for _, g := range groups {
		group, err := parseObject(g)
		if err != nil {
			return nil, fmt.Errorf("parse %s hooks: %w", event, err)
		}
		entries, err := group.array("hooks")
		if err != nil {
			return nil, err
		}
		var kept []json.RawMessage
		for _, h := range entries {
			var e hookEntry
			if json.Unmarshal(h, &e) == nil && isOurs(e.Command) {
				continue
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 {
			continue
		}
		if len(kept) == len(entries) {
			out = append(out, g)
			continue
		}
		if err := group.setArray("hooks", kept); err != nil {
			return nil, err
		}
		data, err := group.MarshalJSON()
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, nil
}

func readSettings(path string) (object, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(bytes.TrimSpace(data)) == 0) {
		return object{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	root, err := parseObject(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return root, nil
}

func edit(change func(hooks *object) error) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	root, err := readSettings(path)
	if err != nil {
		return err
	}
	hooks, err := root.object("hooks")
	if err != nil {
		return err
	}
	if err := change(&hooks); err != nil {
		return err
	}
	if len(hooks) == 0 {
		root.delete("hooks")
	} else {
		data, err := hooks.MarshalJSON()
		if err != nil {
			return err
		}
		root.set("hooks", data)
	}
	raw, err := root.MarshalJSON()
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return fmt.Errorf("format settings: %w", err)
	}
	out.WriteByte('\n')
	return writeFile(path, out.Bytes())
}

// writeFile replaces path atomically, keeping its permissions.
func writeFile(path string, data []byte) error {
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".botler.tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

// commandHook mirrors the field order Claude Code writes, so the added entry reads like the rest of settings.json.
type commandHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func newHookGroup(exe string) map[string][]commandHook {
	return map[string][]commandHook{"hooks": {{Type: "command", Command: HookCommand(exe), Timeout: hookTimeout}}}
}
