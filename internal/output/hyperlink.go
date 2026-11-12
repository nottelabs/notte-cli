package output

import (
	"os"
	"strconv"
	"strings"

	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Link returns label as a clickable link to url when the terminal on stdout
// supports OSC 8 hyperlinks, and url itself otherwise, so the address is
// never lost in a terminal that would show only the label.
func Link(url, label string) string {
	if !hyperlinksSupported(os.Getenv, term.IsTerminal(int(os.Stdout.Fd()))) {
		return url
	}
	return termenv.Hyperlink(url, label)
}

// hyperlinksSupported follows the FORCE_HYPERLINK convention ("1" on, "0"
// off), then recognizes terminals known to render OSC 8 links. Terminals that
// do not understand OSC 8 print the label alone, so unknown ones get the URL.
func hyperlinksSupported(getenv func(string) string, isTerminal bool) bool {
	if force := getenv("FORCE_HYPERLINK"); force != "" {
		return force != "0"
	}
	if !isTerminal || getenv("NO_COLOR") != "" {
		return false
	}
	switch getenv("TERM_PROGRAM") {
	case "iTerm.app", "vscode", "WezTerm", "ghostty", "Hyper", "Tabby", "rio":
		return true
	}
	if getenv("WT_SESSION") != "" || getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if vte, err := strconv.Atoi(getenv("VTE_VERSION")); err == nil && vte >= 5000 {
		return true
	}
	return strings.Contains(getenv("TERM"), "kitty") || strings.Contains(getenv("TERM"), "alacritty")
}
