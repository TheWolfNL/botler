package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TheWolfNL/botler/internal/agent"
	"github.com/TheWolfNL/botler/internal/alert"
	"github.com/TheWolfNL/botler/internal/config"
	"github.com/TheWolfNL/botler/internal/ipc"
)

const (
	bundleID     = "com.github.thewolfnl.botler"
	settingsFlag = "--settings"
	cliUsage     = `usage:
  botler hook [agent]                          read a hook payload on stdin and notify (agent: claude)
  botler notify done|failed|input [message]
  botler hooks install|uninstall|status [agent]
  botler settings
  botler --version`
)

// runCLI handles subcommands; ok is false when args are not a CLI invocation.
func runCLI(args []string) (code int, ok bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "hook":
		return runHook(args[1:]), true
	case "notify":
		return runNotify(args[1:]), true
	case "hooks":
		return runHooks(args[1:]), true
	case "settings":
		return runSettings(), true
	case "version", "-v", "--version":
		fmt.Println("Botler", displayVersion())
		return 0, true
	case "help", "-h", "--help":
		fmt.Println(cliUsage)
		return 0, true
	}
	return 0, false
}

// runHook always exits 0: a failing hook would show an error inside the AI tool.
func runHook(args []string) int {
	id := defaultAgent
	if len(args) > 0 {
		id = args[0]
	}
	a, err := findAgent(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "botler:", err)
		return 0
	}
	e, ok, err := a.ParseHook(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "botler:", err)
		return 0
	}
	if ok {
		deliver(e)
	}
	return 0
}

func runNotify(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, cliUsage)
		return 2
	}
	kind := args[0]
	switch kind {
	case config.KindDone, config.KindFailed, config.KindInput:
	default:
		fmt.Fprintf(os.Stderr, "botler: unknown kind %q\n", kind)
		return 2
	}
	cwd, _ := os.Getwd()
	deliver(ipc.Event{Kind: kind, Project: filepath.Base(cwd), Message: agent.Summarize(strings.Join(args[1:], " "))})
	return 0
}

// deliver hands e to the running app, or starts it and covers this one event with osascript and afplay.
func deliver(e ipc.Event) {
	e.App, e.AppPID = agent.HostApp(os.Getppid())
	if ipc.SendEvent(e) == nil {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "botler:", err)
	}
	if err := exec.Command("open", "-g", "-b", bundleID).Start(); err != nil {
		fmt.Fprintln(os.Stderr, "botler: start app:", err)
	}
	ks := cfg.Kind(e.Kind)
	if !ks.Enabled {
		return
	}
	if ks.Banner {
		text := e.Project
		if e.Message != "" {
			text = strings.TrimPrefix(text+" · "+e.Message, " · ")
		}
		if err := alert.FallbackNotification(alert.Title(e.Kind, e.Agent), text); err != nil {
			fmt.Fprintln(os.Stderr, "botler:", err)
		}
	}
	if ks.Sound {
		if err := alert.PlaySound(ks.SoundName); err != nil {
			fmt.Fprintln(os.Stderr, "botler:", err)
		}
	}
}

func runHooks(args []string) int {
	action := "status"
	if len(args) > 0 {
		action, args = args[0], args[1:]
	}
	selected, err := selectAgents(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "botler:", err)
		return 2
	}
	exe, err := executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "botler:", err)
		return 1
	}
	code := 0
	for _, a := range selected {
		var err error
		switch action {
		case "install":
			if err = a.InstallHooks(exe); err == nil {
				fmt.Printf("%s: hooks installed\n", a.Name())
			}
		case "uninstall", "remove":
			if err = a.UninstallHooks(); err == nil {
				fmt.Printf("%s: hooks removed\n", a.Name())
			}
		case "status":
			var st agent.HookStatus
			if st, err = a.HookStatus(exe); err == nil {
				fmt.Printf("%s: %s\n", a.Name(), describeStatus(st))
			}
		default:
			fmt.Fprintln(os.Stderr, cliUsage)
			return 2
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "botler: %s: %v\n", a.Name(), err)
			code = 1
		}
	}
	return code
}

func describeStatus(st agent.HookStatus) string {
	switch {
	case st.Installed:
		return "installed in " + st.Settings
	case st.Outdated:
		return "outdated: hooks point at another Botler binary; run botler hooks install"
	default:
		return "not installed"
	}
}

// executable resolves symlinks so hooks point at the app bundle, not a Homebrew or ~/.local/bin link.
func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	return resolved, nil
}

func runSettings() int {
	if ipc.Send(ipc.CmdSettings, "") == nil {
		return 0
	}
	if err := exec.Command("open", "-b", bundleID, "--args", settingsFlag).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "botler: launch app:", err)
		return 1
	}
	return 0
}
