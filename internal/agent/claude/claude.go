// Package claude adapts Claude Code to Botler: it reads Claude Code hook payloads and manages Botler's hooks in Claude Code's settings.
package claude

import (
	"io"

	"github.com/TheWolfNL/botler/internal/agent"
	"github.com/TheWolfNL/botler/internal/ipc"
)

// Agent is the Claude Code adapter.
type Agent struct{}

var _ agent.Agent = Agent{}

func (Agent) ID() string    { return "claude" }
func (Agent) Name() string  { return "Claude Code" }
func (Agent) Short() string { return "Claude" }

func (Agent) ParseHook(r io.Reader) (ipc.Event, bool, error) {
	p, err := ReadPayload(r)
	if err != nil {
		return ipc.Event{}, false, err
	}
	e, ok := p.ToEvent()
	return e, ok, nil
}

func (Agent) HookStatus(exe string) (agent.HookStatus, error) { return Status(exe) }
func (Agent) InstallHooks(exe string) error                   { return Install(exe) }
func (Agent) UninstallHooks() error                           { return Uninstall() }
