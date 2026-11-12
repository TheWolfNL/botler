package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

static void cbActiveScreenFrame(double *x, double *w, double *h) {
	NSRect f = [NSScreen mainScreen].frame;
	*x = f.origin.x;
	*w = f.size.width;
	*h = f.size.height;
}

static int cbFrontmostPID(void) {
	NSRunningApplication *app = [NSWorkspace sharedWorkspace].frontmostApplication;
	return app ? app.processIdentifier : 0;
}

static char *cbBundlePath(int pid) {
	NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
	if (!app || !app.bundleURL) return NULL;
	return strdup(app.bundleURL.fileSystemRepresentation);
}
*/
import "C"

import (
	"os/exec"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// activeScreen returns the screen holding the frontmost window, so the banner appears where the user is working.
func activeScreen(app *application.App) *application.Screen {
	var x, w, h C.double
	C.cbActiveScreenFrame(&x, &w, &h)
	for _, s := range app.Screen.GetAll() {
		if s.Bounds.X == int(x) && s.Bounds.Width == int(w) && s.Bounds.Height == int(h) {
			return s
		}
	}
	return nil
}

// frontmostPID returns the pid of the app that has focus.
func frontmostPID() int {
	return int(C.cbFrontmostPID())
}

// activateApp brings the app with pid to the front through LaunchServices, which macOS honours even though the banner never made Botler active.
func activateApp(pid int) bool {
	cpath := C.cbBundlePath(C.int(pid))
	if cpath == nil {
		return false
	}
	path := C.GoString(cpath)
	C.free(unsafe.Pointer(cpath))
	return exec.Command("open", path).Run() == nil
}
