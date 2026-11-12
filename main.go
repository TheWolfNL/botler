package main

import (
	"embed"
	"errors"
	"log"
	"os"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/TheWolfNL/botler/internal/config"
	"github.com/TheWolfNL/botler/internal/ipc"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/tray-template.png
var trayIcon []byte

func main() {
	if code, ok := runCLI(os.Args[1:]); ok {
		os.Exit(code)
	}
	if err := runApp(); err != nil {
		log.Fatal(err)
	}
}

func runApp() error {
	alerter, err := newAlerter()
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name:        "Botler",
		Description: "Tells you when your AI coding agent is done or needs you",
		Services: []application.Service{
			application.NewService(&SettingsService{alerter: alerter}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	settings := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "settings",
		Title:         "Botler",
		Width:         900,
		Height:        640,
		Hidden:        true,
		DisableResize: true,
		URL:           "/",
	})
	settings.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		settings.Hide()
		e.Cancel()
	})

	tray := app.SystemTray.New()
	tray.SetTemplateIcon(trayIcon)
	tray.SetTooltip("Botler")
	alerter.tray = tray
	alerter.banners = &Banners{app: app}

	showSettings := func() {
		settings.Show()
		settings.Focus()
	}
	test := func(kind string) func(*application.Context) {
		return func(*application.Context) {
			(&SettingsService{alerter: alerter}).Test(kind)
		}
	}
	menu := app.NewMenu()
	menu.AddSectionHeader("Botler")
	testMenu := menu.AddSubmenu("Test notification")
	testMenu.Add("Done").OnClick(test(config.KindDone))
	testMenu.Add("Failed").OnClick(test(config.KindFailed))
	testMenu.Add("Needs input").OnClick(test(config.KindInput))
	menu.Add("Dismiss all").OnClick(func(*application.Context) { alerter.banners.DismissAll() })
	menu.Add("Settings…").OnClick(func(*application.Context) { showSettings() })
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)

	ln, err := ipc.Listen(func(cmd, arg string) {
		switch cmd {
		case ipc.CmdNotify:
			e, err := ipc.ParseEvent(arg)
			if err != nil {
				log.Print(err)
				return
			}
			alerter.Trigger(e)
		case ipc.CmdSettings:
			showSettings()
		}
	})
	if errors.Is(err, ipc.ErrAlreadyRunning) {
		log.Print(err)
		return nil
	}
	if err != nil {
		return err
	}
	defer ln.Close()

	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { showSettings() })

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		alerter.RevealOnStartup()
		if slices.Contains(os.Args[1:], settingsFlag) || needsHooksPrompt(alerter.Config()) {
			showSettings()
		}
	})

	return app.Run()
}

// needsHooksPrompt reports whether Settings should open at launch to offer installing hooks for an AI tool.
func needsHooksPrompt(cfg config.Config) bool {
	if cfg.HooksPrompted {
		return false
	}
	exe, err := executable()
	if err != nil {
		return false
	}
	for _, a := range agents {
		if st, err := a.HookStatus(exe); err == nil && !st.Installed {
			return true
		}
	}
	return false
}
