// Package agent defines what Botler needs from an AI coding tool: reading its hook payloads and managing its hooks.
package agent

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/TheWolfNL/botler/internal/ipc"
)

// Agent adapts one AI coding tool, such as Claude Code, to Botler.
type Agent interface {
	// ID names the agent in hook commands and on the CLI, e.g. "claude".
	ID() string
	// Name is the tool's display name, e.g. "Claude Code".
	Name() string
	// Short names the agent in banner titles, e.g. "Claude" in "Claude is done".
	Short() string
	// ParseHook turns one hook payload into an event; ok is false for payloads that should stay silent.
	ParseHook(r io.Reader) (e ipc.Event, ok bool, err error)
	// HookStatus reports whether the tool's hooks run exe.
	HookStatus(exe string) (HookStatus, error)
	InstallHooks(exe string) error
	UninstallHooks() error
}

// HookStatus describes Botler's hooks in one tool's settings.
type HookStatus struct {
	Installed bool `json:"installed"`
	// Outdated means some hooks point at another Botler binary.
	Outdated bool   `json:"outdated"`
	Settings string `json:"settings"`
}

const maxMessage = 280

// Summarize collapses whitespace and markdown emphasis and trims text to fit a banner.
func Summarize(s string) string {
	s = strings.NewReplacer("**", "", "`", "", "#", "").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= maxMessage {
		return s
	}
	r := []rune(s)[:maxMessage-1]
	return strings.TrimSpace(string(r)) + "…"
}
