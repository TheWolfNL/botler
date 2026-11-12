package claude

import (
	"strings"
	"testing"
)

func TestToEvent(t *testing.T) {
	tests := []struct {
		in       string
		kind     string
		message  string
		silenced bool
	}{
		{in: `{"hook_event_name":"Stop","cwd":"/src/api","last_assistant_message":"**Done.**\n\nAll   green"}`, kind: "done", message: "Done. All green"},
		{in: `{"hook_event_name":"StopFailure","cwd":"/src/api","error":"rate_limit","error_details":"429 Too Many Requests"}`, kind: "failed", message: "429 Too Many Requests"},
		{in: `{"hook_event_name":"StopFailure","error":"unknown"}`, kind: "failed", message: "unknown"},
		{in: `{"hook_event_name":"Notification","cwd":"/src/api","message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`, kind: "input", message: "Claude needs your permission to use Bash"},
		{in: `{"hook_event_name":"Notification","notification_type":"auth_success"}`, silenced: true},
		{in: `{"hook_event_name":"PreToolUse"}`, silenced: true},
	}
	for _, tt := range tests {
		p, err := ReadPayload(strings.NewReader(tt.in))
		if err != nil {
			t.Fatal(err)
		}
		e, ok := p.ToEvent()
		if ok == tt.silenced {
			t.Errorf("%s: ok = %v", tt.in, ok)
			continue
		}
		if e.Kind != tt.kind || e.Message != tt.message {
			t.Errorf("%s: got %q %q, want %q %q", tt.in, e.Kind, e.Message, tt.kind, tt.message)
		}
		if !tt.silenced && e.Agent != "Claude" {
			t.Errorf("%s: agent = %q", tt.in, e.Agent)
		}
		if strings.Contains(tt.in, "/src/api") && e.Project != "api" {
			t.Errorf("%s: project = %q", tt.in, e.Project)
		}
	}
}
