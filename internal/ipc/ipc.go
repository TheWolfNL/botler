// Package ipc carries requests from the CLI to the running app over a unix socket.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dialTimeout = 300 * time.Millisecond
	maxLineLen  = 4096
)

// Commands understood by the app.
const (
	CmdNotify   = "notify"
	CmdSettings = "settings"
)

// SocketPath returns ~/Library/Application Support/botler/notify.sock.
func SocketPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(dir, "botler", "notify.sock"), nil
}

// ErrAlreadyRunning is returned by Listen when another app instance owns the socket.
var ErrAlreadyRunning = errors.New("another Botler instance is running")

// Send delivers a command and its argument to the running app.
func Send(cmd, arg string) error {
	p, err := SocketPath()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("unix", p, dialTimeout)
	if err != nil {
		return fmt.Errorf("dial app: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(dialTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	if _, err := fmt.Fprintln(conn, sanitize(cmd+" "+arg)); err != nil {
		return fmt.Errorf("send %s: %w", cmd, err)
	}
	return nil
}

// Listen serves the socket until the returned listener is closed, calling handle per request.
func Listen(handle func(cmd, arg string)) (net.Listener, error) {
	p, err := SocketPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}
	if conn, err := net.DialTimeout("unix", p, dialTimeout); err == nil {
		conn.Close()
		return nil, ErrAlreadyRunning
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", p)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	go serve(ln, handle)
	return ln, nil
}

func serve(ln net.Listener, handle func(cmd, arg string)) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			line, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil && line == "" {
				return
			}
			cmd, arg, _ := strings.Cut(sanitize(line), " ")
			handle(cmd, arg)
		}()
	}
}

func sanitize(line string) string {
	line = strings.TrimSpace(strings.ReplaceAll(line, "\n", " "))
	if len(line) > maxLineLen {
		line = line[:maxLineLen]
	}
	return line
}

// Event is the payload of a notify request: one AI tool hook outcome.
type Event struct {
	Kind string `json:"kind"`
	// Agent is the short name of the AI tool that sent the event, e.g. "Claude".
	Agent   string `json:"agent,omitempty"`
	Session string `json:"session,omitempty"`
	Project string `json:"project,omitempty"`
	Message string `json:"message,omitempty"`
	// App and AppPID name the GUI app hosting the Claude session, so a click can bring it forward.
	App    string `json:"app,omitempty"`
	AppPID int    `json:"appPid,omitempty"`
}

// SendEvent asks the running app to announce e.
func SendEvent(e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return Send(CmdNotify, string(data))
}

// ParseEvent decodes a notify argument.
func ParseEvent(arg string) (Event, error) {
	var e Event
	if err := json.Unmarshal([]byte(arg), &e); err != nil {
		return Event{}, fmt.Errorf("decode event: %w", err)
	}
	return e, nil
}
