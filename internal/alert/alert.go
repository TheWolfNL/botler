// Package alert holds the alert channels that need no running app: sound and the osascript fallback.
package alert

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/TheWolfNL/botler/internal/config"
)

// Title returns the headline for a notification kind sent by agent, e.g. "Claude is done".
func Title(kind, agent string) string {
	if agent == "" {
		agent = "Your agent"
	}
	switch kind {
	case config.KindFailed:
		return agent + " hit an error"
	case config.KindInput:
		return agent + " needs you"
	default:
		return agent + " is done"
	}
}

// PlaySound starts afplay without waiting for it to finish.
func PlaySound(name string) error {
	cmd := exec.Command("afplay", config.SoundPath(name))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start afplay: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// FallbackNotification posts a notification via osascript, used when the app is not running.
func FallbackNotification(title, text string) error {
	script := fmt.Sprintf("display notification %s with title %s",
		strconv.Quote(text), strconv.Quote(title))
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start osascript: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
