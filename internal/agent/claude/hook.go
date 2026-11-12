package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/TheWolfNL/botler/internal/agent"
	"github.com/TheWolfNL/botler/internal/config"
	"github.com/TheWolfNL/botler/internal/ipc"
)

const maxPayload = 4 << 20

// Payload holds the hook input fields Botler uses; see https://code.claude.com/docs/en/hooks.
type Payload struct {
	Event            string `json:"hook_event_name"`
	Session          string `json:"session_id"`
	Cwd              string `json:"cwd"`
	Message          string `json:"message"`
	NotificationType string `json:"notification_type"`
	LastMessage      string `json:"last_assistant_message"`
	Error            string `json:"error"`
	ErrorDetails     string `json:"error_details"`
}

// ReadPayload decodes a hook payload from r.
func ReadPayload(r io.Reader) (Payload, error) {
	var p Payload
	if err := json.NewDecoder(io.LimitReader(r, maxPayload)).Decode(&p); err != nil {
		return Payload{}, fmt.Errorf("decode hook payload: %w", err)
	}
	return p, nil
}

// ToEvent maps a payload to a notification; ok is false for hook events that should stay silent.
func (p Payload) ToEvent() (e ipc.Event, ok bool) {
	e = ipc.Event{Agent: Agent{}.Short(), Session: p.Session}
	if p.Cwd != "" {
		e.Project = filepath.Base(p.Cwd)
	}
	switch p.Event {
	case "Stop":
		e.Kind = config.KindDone
		e.Message = p.LastMessage
	case "StopFailure":
		e.Kind = config.KindFailed
		e.Message = firstNonEmpty(p.ErrorDetails, p.Error, p.LastMessage)
	case "Notification":
		if p.NotificationType == "auth_success" {
			return ipc.Event{}, false
		}
		e.Kind = config.KindInput
		e.Message = p.Message
	default:
		return ipc.Event{}, false
	}
	e.Message = agent.Summarize(e.Message)
	return e, true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
