package rfd

import (
	"regexp"
	"strings"
)

// A checklistFormat is a markup language an implementation checklist can be
// written in.
type checklistFormat struct {
	name     string // file name
	title    string // title prefix, before "RFD NNNN implementation checklist"
	backlink *regexp.Regexp
	// links rewrites the format's links into AsciiDoc.
	links []rewrite
}

type rewrite struct {
	pattern     *regexp.Regexp
	replacement string
}

var checklistFormats = []checklistFormat{
	{
		name:     "IMPLEMENTATION.adoc",
		title:    "= ",
		backlink: regexp.MustCompile(`link:README\.adoc\[[^\]]+\]`),
	},
	{
		name:     "IMPLEMENTATION.org",
		title:    "#+TITLE: ",
		backlink: regexp.MustCompile(`\[\[file:README\.adoc\]\[[^\]]+\]\]`),
		links: []rewrite{
			{regexp.MustCompile(`\[\[file:([^\]]*)\]\[([^\]]*)\]\]`), "link:$1[$2]"},
			{regexp.MustCompile(`^\* ([^\[])`), "== $1"},
		},
	},
	{
		name:     "IMPLEMENTATION.md",
		title:    "# ",
		backlink: regexp.MustCompile(`\[[^\]]+\]\(README\.adoc\)`),
		links: []rewrite{
			{regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`), "link:$2[$1]"},
		},
	},
}

// heading returns the first line a checklist for entry must have.
func (f checklistFormat) heading(entry string) string {
	return f.title + "RFD " + entry + " implementation checklist"
}

// toAsciiDoc rewrites a checklist's title and links into AsciiDoc, so that
// equivalent checklists in different formats compare equal.
func (f checklistFormat) toAsciiDoc(text string) []string {
	title := regexp.MustCompile(`^` + regexp.QuoteMeta(f.title) + `(RFD [0-9]+ implementation checklist)$`)
	out := lines(text)
	for i := range out {
		if i == 0 {
			out[i] = title.ReplaceAllString(out[i], "= $1")
		}
		for _, r := range f.links {
			out[i] = r.pattern.ReplaceAllString(out[i], r.replacement)
		}
	}
	return out
}

var (
	task          = regexp.MustCompile(`^([[:space:]]*[-+*]|[[:space:]]*[0-9]+\.)[[:space:]]+\[[Xx ]\][[:space:]]+[^[:space:]]`)
	taskDone      = regexp.MustCompile(`^([[:space:]]*[-+*]|[[:space:]]*[0-9]+\.)[[:space:]]+\[[Xx]\]`)
	malformedTask = regexp.MustCompile(`^([[:space:]]*[-+*]|[[:space:]]*[0-9]+\.)[[:space:]]+\[[^\]]*\]([[:space:]]|$)`)
	taskBullet    = regexp.MustCompile(`^([[:space:]]*)[-+]([[:space:]]+\[)`)

	orgBlockStart   = regexp.MustCompile(`^[[:space:]]*#\+begin_`)
	orgBlockEnd     = regexp.MustCompile(`^[[:space:]]*#\+end_`)
	markdownFence   = regexp.MustCompile("^[[:space:]]*(```|~~~)")
	asciidocDelimit = regexp.MustCompile(`^[[:space:]]*(----|\.\.\.\.|====|____|\*\*\*\*)[[:space:]]*$`)
)

// taskLines returns the lines of text outside literal blocks, where a task
// would be a real task rather than an example. Org blocks and Markdown fences
// are always literal; AsciiDoc delimited blocks are skipped when asciidoc is
// set.
func taskLines(text string, asciidoc bool) []string {
	var out []string
	orgBlock, fence, block := 0, false, false
	for _, line := range lines(text) {
		lower := strings.ToLower(line)
		switch {
		case orgBlockStart.MatchString(lower):
			orgBlock++
		case orgBlockEnd.MatchString(lower):
			if orgBlock > 0 {
				orgBlock--
			}
		case orgBlock > 0:
		case markdownFence.MatchString(line):
			fence = !fence
		case fence:
		case asciidoc && asciidocDelimit.MatchString(line):
			block = !block
		case block:
		default:
			out = append(out, line)
		}
	}
	return out
}

// countTasks counts the complete, total, and malformed tasks in a checklist.
func countTasks(text string) (complete, total, malformed int) {
	for _, line := range taskLines(text, false) {
		switch {
		case task.MatchString(line):
			total++
			if taskDone.MatchString(line) {
				complete++
			}
		case malformedTask.MatchString(line):
			malformed++
		}
	}
	return complete, total, malformed
}

// countDesignTasks counts tasks in an RFD's README, where they do not belong.
func countDesignTasks(text string) int {
	n := 0
	for _, line := range taskLines(text, true) {
		if task.MatchString(line) {
			n++
		}
	}
	return n
}

var (
	checklistLink = regexp.MustCompile(`link:IMPLEMENTATION\.(org|md)\[`)
	mutableLine   = regexp.MustCompile(`^:(labels|visibility):`)
)

// canonicalText returns an RFD's content in a form where only changes to the
// decision differ. Final RFD content is immutable, but moving the
// implementation checklist between formats is a lossless presentation change,
// and labels and visibility are about finding and sharing an RFD rather than
// part of the decision, so retagging or publishing a final RFD is allowed. So
// is linking a mention of another RFD, or rewrapping lines.
func canonicalText(tree Tree, entry string) string {
	var b strings.Builder
	if readme, err := tree.ReadFile(entry + "/README.adoc"); err == nil {
		for _, line := range lines(string(readme)) {
			if mutableLine.MatchString(line) {
				continue
			}
			b.WriteString(unlinkRFDs(checklistLink.ReplaceAllString(line, "link:IMPLEMENTATION.adoc[")))
			b.WriteByte('\n')
		}
	}
	for _, format := range checklistFormats {
		checklist, err := tree.ReadFile(entry + "/" + format.name)
		if err != nil {
			continue
		}
		for _, line := range format.toAsciiDoc(string(checklist)) {
			b.WriteString(unlinkRFDs(taskBullet.ReplaceAllString(line, "$1*$2")))
			b.WriteByte('\n')
		}
		break
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
