package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/TheWolfNL/botler/internal/alert"
	"github.com/TheWolfNL/botler/internal/config"
	"github.com/TheWolfNL/botler/internal/ipc"
)

const (
	bannerWidth    = 400
	bannerHeight   = 104
	bannerMargin   = 8
	bannerHideAnim = 300 * time.Millisecond
	maxBanners     = 4
)

type banner struct {
	id     int
	event  ipc.Event
	win    *application.WebviewWindow
	screen *application.Screen
	timer  *time.Timer
}

// Banners keeps a stack of notification panels on the active screen, one per Claude session.
type Banners struct {
	app *application.App

	mu     sync.Mutex
	items  []*banner
	pos    string
	nextID int
}

// Show puts a banner for e on top of the stack, replacing an older one from the same session; autoHide 0 keeps it until dismissed.
func (b *Banners) Show(e ipc.Event, pos string, autoHide int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pos = pos

	for _, it := range b.items {
		if e.Session != "" && it.event.Session == e.Session {
			b.removeLocked(it, false)
			break
		}
	}
	if len(b.items) >= maxBanners {
		b.removeLocked(b.items[len(b.items)-1], false)
	}

	b.nextID++
	id := b.nextID
	q := url.Values{
		"id":       {strconv.Itoa(id)},
		"kind":     {e.Kind},
		"title":    {alert.Title(e.Kind, e.Agent)},
		"project":  {e.Project},
		"message":  {e.Message},
		"app":      {e.App},
		"pos":      {pos},
		"autoHide": {strconv.Itoa(autoHide)},
	}
	screen := activeScreen(b.app)
	if screen == nil {
		screen = b.app.Screen.GetPrimary()
	}
	win := b.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             fmt.Sprintf("banner-%d", id),
		Width:            bannerWidth,
		Height:           bannerHeight,
		Hidden:           true,
		Frameless:        true,
		DisableResize:    true,
		Screen:           screen,
		URL:              "/#banner?" + q.Encode(),
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		KeyBindings: map[string]func(application.Window){
			"Escape": func(application.Window) { go b.Dismiss(id) },
		},
		Mac: application.MacWindow{
			Backdrop:      application.MacBackdropTransparent,
			DisableShadow: true,
			WindowClass:   application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				NonActivating: true,
				FloatingPanel: true,
			},
			WindowLevel: application.MacWindowLevelPopUpMenu,
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary |
				application.MacWindowCollectionBehaviorStationary |
				application.MacWindowCollectionBehaviorIgnoresCycle,
		},
	})
	it := &banner{id: id, event: e, win: win, screen: screen}
	b.items = append([]*banner{it}, b.items...)
	b.layoutLocked()
	win.Show()
	if autoHide > 0 {
		it.timer = time.AfterFunc(time.Duration(autoHide)*time.Second, func() { b.Dismiss(id) })
	}
}

// Dismiss plays the exit animation of banner id, then closes it and closes the gap in the stack.
func (b *Banners) Dismiss(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if it := b.findLocked(id); it != nil {
		b.removeLocked(it, true)
		b.layoutLocked()
	}
}

// Click brings the app hosting the banner's Claude session forward, then dismisses the banner.
func (b *Banners) Click(id int) {
	b.mu.Lock()
	it := b.findLocked(id)
	b.mu.Unlock()
	if it == nil {
		return
	}
	if it.event.AppPID != 0 {
		activateApp(it.event.AppPID)
	}
	b.Dismiss(id)
}

// DismissAll clears the stack, used by the menu.
func (b *Banners) DismissAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.items) > 0 {
		b.removeLocked(b.items[0], true)
	}
}

func (b *Banners) findLocked(id int) *banner {
	for _, it := range b.items {
		if it.id == id {
			return it
		}
	}
	return nil
}

func (b *Banners) removeLocked(it *banner, animate bool) {
	for i, x := range b.items {
		if x == it {
			b.items = append(b.items[:i], b.items[i+1:]...)
			break
		}
	}
	if it.timer != nil {
		it.timer.Stop()
	}
	if !animate {
		it.win.Close()
		return
	}
	it.win.ExecJS(`document.body.classList.add("leaving")`)
	time.AfterFunc(bannerHideAnim, it.win.Close)
}

// layoutLocked stacks banners away from the anchor corner, newest closest to it.
func (b *Banners) layoutLocked() {
	step := bannerHeight - 10
	for i, it := range b.items {
		x, y := bannerOrigin(it.screen, b.pos)
		if strings.HasPrefix(b.pos, "bottom") {
			y -= i * step
		} else {
			y += i * step
		}
		it.win.SetRelativePosition(x, y)
	}
}

// bannerOrigin returns the banner's top-left corner relative to screen, kept inside the area not covered by the menu bar and Dock.
func bannerOrigin(screen *application.Screen, pos string) (x, y int) {
	if screen == nil {
		return 0, 0
	}
	wa := screen.WorkArea
	left := wa.X - screen.Bounds.X + bannerMargin
	right := wa.X - screen.Bounds.X + wa.Width - bannerWidth - bannerMargin
	top := wa.Y - screen.Bounds.Y + bannerMargin
	bottom := wa.Y - screen.Bounds.Y + wa.Height - bannerHeight - bannerMargin

	switch pos {
	case config.BannerTopLeft:
		return left, top
	case config.BannerTopCenter:
		return (screen.Bounds.Width - bannerWidth) / 2, top
	case config.BannerBottomLeft:
		return left, bottom
	case config.BannerBottomRight:
		return right, bottom
	default:
		return right, top
	}
}
