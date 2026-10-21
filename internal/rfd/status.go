package rfd

import (
	"fmt"
	"io"
	"strings"
)

// Percent returns how much of the implementation checklist is done, rounded
// down so that an RFD reads 100% only when every task is.
func (r RFD) Percent() int {
	if r.Total == 0 {
		return 0
	}
	return r.Complete * 100 / r.Total
}

// Palette colors terminal output. The zero value prints plain text.
type Palette struct {
	Reset, Bold, Red, Green, Yellow, Blue, Dim string
}

// ANSI is a Palette of ANSI terminal escapes.
var ANSI = Palette{
	Reset:  "\033[0m",
	Bold:   "\033[1m",
	Red:    "\033[31m",
	Green:  "\033[32m",
	Yellow: "\033[33m",
	Blue:   "\033[34m",
	Dim:    "\033[2m",
}

func (p Palette) state(state string) string {
	switch state {
	case "prediscussion", "ideation":
		return p.Blue
	case "discussion":
		return p.Yellow
	case "published", "committed":
		return p.Green
	case "abandoned":
		return p.Dim
	}
	return p.Red
}

func (p Palette) progress(r RFD) string {
	switch {
	case r.Total == 0 || r.Complete == 0:
		return p.Dim
	case r.Complete == r.Total:
		return p.Green
	}
	return p.Yellow
}

// WriteStatus prints a table of RFDs with their state and implementation
// progress.
func WriteStatus(w io.Writer, rfds []RFD, p Palette) {
	labelWidth := len("Labels")
	for _, r := range rfds {
		labelWidth = max(labelWidth, len(or(r.Labels, "(missing labels)")))
	}
	row := "%-4s  %-13s  %-8s  %-*s  %s"
	fmt.Fprintf(w, p.Bold+row+p.Reset+"\n", "RFD", "State", "Progress", labelWidth, "Labels", "Title")
	fmt.Fprintf(w, p.Dim+row+p.Reset+"\n", "----", strings.Repeat("-", 13), strings.Repeat("-", 8), labelWidth, strings.Repeat("-", labelWidth), strings.Repeat("-", 30))
	for _, r := range rfds {
		progress := "-"
		if r.Total > 0 {
			progress = fmt.Sprintf("%d%%", r.Percent())
		}
		fmt.Fprintf(w, "%-4s  %s%-13s%s  %s%-8s%s  %-*s  %s\n",
			r.Entry,
			p.state(r.State), or(r.State, "(missing)"), p.Reset,
			p.progress(r), progress, p.Reset,
			labelWidth, or(r.Labels, "(missing labels)"),
			or(r.Title, "(missing title)"))
	}
}

// WriteIssues prints each issue, then whether the check passed.
func WriteIssues(w io.Writer, issues []Issue, p Palette) {
	for _, issue := range issues {
		fmt.Fprintf(w, "%s%s%s: %s\n", p.Red, issue.Path, p.Reset, issue.Message)
	}
	if len(issues) > 0 {
		fmt.Fprintf(w, "\n%sRFD status check failed%s with %d issue(s).\n", p.Red, p.Reset, len(issues))
		return
	}
	fmt.Fprintf(w, "%sRFD status check passed.%s\n", p.Green, p.Reset)
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
