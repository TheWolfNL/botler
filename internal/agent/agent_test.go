package agent

import (
	"strings"
	"testing"
)

func TestSummarizeTruncates(t *testing.T) {
	s := Summarize(strings.Repeat("é", 400))
	if r := []rune(s); len(r) != maxMessage || r[len(r)-1] != '…' {
		t.Errorf("len %d, last %q", len(r), r[len(r)-1])
	}
}

func TestSummarizeCollapses(t *testing.T) {
	if got := Summarize("**Done.**\n\n`go test`   passes"); got != "Done. go test passes" {
		t.Errorf("Summarize = %q", got)
	}
}

func TestBundleName(t *testing.T) {
	tests := map[string]string{
		"/Applications/Ghostty.app/Contents/MacOS/ghostty":                                                                      "Ghostty",
		"/Applications/Visual Studio Code.app/Contents/MacOS/Electron":                                                          "Visual Studio Code",
		"/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper (Plugin).app/Contents/MacOS/Code Helper (Plugin)": "",
		"/bin/zsh": "",
	}
	for in, want := range tests {
		if got := bundleName(in); got != want {
			t.Errorf("bundleName(%q) = %q, want %q", in, got, want)
		}
	}
}
