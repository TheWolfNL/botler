package main

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/TheWolfNL/botler/internal/agent"
	"github.com/TheWolfNL/botler/internal/alert"
	"github.com/TheWolfNL/botler/internal/config"
	"github.com/TheWolfNL/botler/internal/ipc"
)

// SettingsService is the frontend's API for reading and changing settings.
type SettingsService struct {
	alerter *Alerter
}

func (s *SettingsService) Get() config.Config {
	return s.alerter.Config()
}

func (s *SettingsService) Save(cfg config.Config) error {
	return s.alerter.SetConfig(cfg)
}

// SoundList groups the selectable alert sounds.
type SoundList struct {
	Custom []string `json:"custom"`
	System []string `json:"system"`
}

func (s *SettingsService) Sounds() (SoundList, error) {
	system, err := config.SystemSounds()
	if err != nil {
		return SoundList{}, err
	}
	custom, err := config.CustomSounds()
	if err != nil {
		return SoundList{}, err
	}
	return SoundList{Custom: custom, System: system}, nil
}

// AddSound lets the user pick an audio file and returns its sound name, or "" when cancelled.
func (s *SettingsService) AddSound() (string, error) {
	dlg := application.Get().Dialog.OpenFile().
		SetTitle("Choose a notification sound").
		CanChooseFiles(true).
		AddFilter("Audio", "*"+strings.Join(config.AudioExtensions, ";*"))
	path, err := dlg.PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	return config.ImportSound(path)
}

func (s *SettingsService) RemoveSound(name string) error {
	return config.RemoveSound(name)
}

func (s *SettingsService) PreviewSound(name string) error {
	return alert.PlaySound(name)
}

var testMessages = map[string]string{
	config.KindDone:   "All tests pass. I've pushed the branch and opened a draft PR.",
	config.KindFailed: "API Error: 529 Overloaded. Try again in a moment.",
	config.KindInput:  "Claude needs your permission to use Bash",
}

// Test shows a sample notification of kind, skipping the "only when away" check.
func (s *SettingsService) Test(kind string) {
	s.alerter.Trigger(ipc.Event{Kind: kind, Agent: agents[0].Short(), Session: "test-" + kind, Project: "botler", Message: testMessages[kind]})
}

func (s *SettingsService) ClickBanner(id int) {
	if s.alerter.banners != nil {
		s.alerter.banners.Click(id)
	}
}

func (s *SettingsService) DismissBanner(id int) {
	if s.alerter.banners != nil {
		s.alerter.banners.Dismiss(id)
	}
}

// AgentInfo describes one supported AI tool and the state of its Botler hooks.
type AgentInfo struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Status agent.HookStatus `json:"status"`
	// Error explains why the tool's settings could not be read.
	Error string `json:"error,omitempty"`
}

func agentInfo(a agent.Agent) AgentInfo {
	info := AgentInfo{ID: a.ID(), Name: a.Name()}
	exe, err := executable()
	if err == nil {
		info.Status, err = a.HookStatus(exe)
	}
	if err != nil {
		info.Error = err.Error()
	}
	return info
}

// Agents lists every supported AI tool with its hook status.
func (s *SettingsService) Agents() []AgentInfo {
	out := make([]AgentInfo, 0, len(agents))
	for _, a := range agents {
		out = append(out, agentInfo(a))
	}
	return out
}

func (s *SettingsService) InstallHooks(id string) (AgentInfo, error) {
	a, err := findAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}
	exe, err := executable()
	if err != nil {
		return AgentInfo{}, err
	}
	if err := a.InstallHooks(exe); err != nil {
		return AgentInfo{}, err
	}
	if err := s.markHooksPrompted(); err != nil {
		return AgentInfo{}, err
	}
	return agentInfo(a), nil
}

func (s *SettingsService) UninstallHooks(id string) (AgentInfo, error) {
	a, err := findAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}
	if err := a.UninstallHooks(); err != nil {
		return AgentInfo{}, err
	}
	return agentInfo(a), nil
}

// RequestAgent opens a GitHub issue form for asking support for another AI tool.
func (s *SettingsService) RequestAgent() error {
	return application.Get().Browser.OpenURL(requestAgentURL)
}

// DeclineHooks records that the first-launch hooks prompt was answered with "Not now".
func (s *SettingsService) DeclineHooks() error {
	return s.markHooksPrompted()
}

func (s *SettingsService) markHooksPrompted() error {
	cfg := s.alerter.Config()
	if cfg.HooksPrompted {
		return nil
	}
	cfg.HooksPrompted = true
	return s.alerter.SetConfig(cfg)
}

func (s *SettingsService) Version() string { return displayVersion() }

// FitHeight resizes the settings window to its content, capped to the screen it is on.
func (s *SettingsService) FitHeight(content, viewport int) {
	win, ok := application.Get().Window.GetByName("settings")
	if !ok || content <= 0 || viewport <= 0 {
		return
	}
	w, h := win.Size()
	h += content - viewport
	// Leave a margin on short screens; the page scrolls when it is cut off.
	if screen, err := win.GetScreen(); err == nil && screen != nil && h > screen.WorkArea.Height-40 {
		h = screen.WorkArea.Height - 40
	}
	win.SetSize(w, h)
	win.Center()
}

func (s *SettingsService) LaunchAtLogin() (bool, error) {
	return application.Get().Autostart.IsEnabled()
}

func (s *SettingsService) SetLaunchAtLogin(enabled bool) error {
	if enabled {
		return application.Get().Autostart.Enable()
	}
	return application.Get().Autostart.Disable()
}
