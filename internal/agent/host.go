package agent

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const maxDepth = 30

// HostApp walks the ancestors of pid to the outermost macOS app bundle (Terminal, iTerm, VS Code, …) and returns its name and pid.
func HostApp(pid int) (name string, appPID int) {
	procs, err := processTable()
	if err != nil {
		return "", 0
	}
	for depth := 0; depth < maxDepth && pid > 1; depth++ {
		p, ok := procs[pid]
		if !ok {
			break
		}
		if n := bundleName(p.exe); n != "" {
			name, appPID = n, pid
		}
		pid = p.ppid
	}
	return name, appPID
}

// bundleName returns "Ghostty" for ".../Ghostty.app/Contents/MacOS/ghostty", skipping helper apps nested inside another bundle.
func bundleName(exe string) string {
	app, _, ok := strings.Cut(exe, ".app/Contents/MacOS/")
	if !ok || strings.Contains(app, ".app/") {
		return ""
	}
	return filepath.Base(app)
}

type proc struct {
	ppid int
	exe  string
}

func processTable() (map[int]proc, error) {
	out, err := exec.Command("ps", "-Ao", "pid=,ppid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	procs := make(map[int]proc)
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		pidStr, rest, _ := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		ppidStr, exe, _ := strings.Cut(strings.TrimSpace(rest), " ")
		pid, err1 := strconv.Atoi(pidStr)
		ppid, err2 := strconv.Atoi(ppidStr)
		if err1 != nil || err2 != nil {
			continue
		}
		procs[pid] = proc{ppid: ppid, exe: strings.TrimSpace(exe)}
	}
	return procs, sc.Err()
}
