package rfd

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// An RFD is the status of one RFD.
type RFD struct {
	Entry      string // directory name, such as "0001"
	Title      string
	State      string
	Labels     string
	Visibility string // public or private
	Complete   int    // implementation tasks done
	Total      int    // implementation tasks
}

// An Issue is a problem with the RFDs.
type Issue struct {
	Path    string // RFD entry or file the issue is about
	Message string
}

func (i Issue) String() string {
	return i.Path + ": " + i.Message
}

// A Report is the status of every RFD and the problems found with them.
type Report struct {
	RFDs   []RFD
	Issues []Issue
}

func (r *Report) problem(path, format string, args ...any) {
	r.Issues = append(r.Issues, Issue{Path: path, Message: fmt.Sprintf(format, args...)})
}

var (
	entryName = regexp.MustCompile(`^[0-9]{4}$`)
	rfdLink   = regexp.MustCompile(`link:\.\./([0-9]{4})/`)
)

// An rfdLinkTarget is a link from one of an RFD's documents to another RFD.
type rfdLinkTarget struct {
	doc    string // README.adoc or IMPLEMENTATION.adoc
	target string // RFD entry linked to
}

// Check reads the RFDs in dir and validates them. When base is not nil, it
// also checks the changes from base: state transitions, deleted RFDs, and
// edits to final RFDs. RFDs whose project, their first label, is one of
// private are private unless set otherwise, and must never be public.
func Check(dir string, base Tree, private []string) (*Report, error) {
	current := DirTree(dir)
	names, err := current.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("RFD directory not found: %s", dir)
	}
	if base != nil {
		baseNames, err := base.ReadDir(".")
		if err != nil && !notExist(err) {
			return nil, err
		}
		names = append(names, baseNames...)
	}
	slices.Sort(names)
	names = slices.Compact(names)

	report := &Report{}
	found := false
	links := map[string][]rfdLinkTarget{} // RFD entry to the RFDs its documents link to
	for _, name := range names {
		// The old RFD index, still in older base revisions, is not an RFD.
		if name == "README.adoc" || name == "README.md" {
			continue
		}

		previous := ""
		if base != nil {
			previous = baseState(base, name)
		}

		files, err := current.ReadDir(name)
		if notExist(err) && previous != "" {
			report.problem(name, "historical RFD must not be deleted")
			found = true
			continue
		}
		if err != nil || !entryName.MatchString(name) {
			report.problem(name, "invalid RFD entry")
			continue
		}
		found = true

		rfd, ok := checkEntry(report, current, name, files, private)
		if !ok {
			continue
		}
		if previous != "" && !validTransition(previous, rfd.State) {
			report.problem(name, "invalid RFD state transition from %s to %s", previous, rfd.State)
		}
		if final(previous) && canonicalText(current, name) != canonicalText(base, name) {
			report.problem(name, "final RFD content is immutable (%s)", previous)
		}
		report.RFDs = append(report.RFDs, rfd)
		for _, doc := range []string{"README.adoc", "IMPLEMENTATION.adoc"} {
			text, err := current.ReadFile(name + "/" + doc)
			if err != nil {
				continue
			}
			for _, m := range rfdLink.FindAllStringSubmatch(string(text), -1) {
				links[name] = append(links[name], rfdLinkTarget{doc, m[1]})
			}
		}
	}

	checkAllLinks(report, current, report.RFDs)

	// A public RFD must make sense without the private ones.
	visibility := map[string]string{}
	for _, r := range report.RFDs {
		visibility[r.Entry] = r.Visibility
	}
	for _, r := range report.RFDs {
		if r.Visibility != "public" {
			continue
		}
		for _, l := range links[r.Entry] {
			if visibility[l.target] == "private" {
				report.problem(r.Entry+"/"+l.doc, "public RFD links to private RFD %s", l.target)
			}
		}
	}
	if !found {
		report.problem(dir, "no RFDs found")
	}
	return report, nil
}

// checkEntry validates one RFD directory and reads its status. It reports
// false when the RFD has no README.adoc to read.
func checkEntry(report *Report, tree Tree, name string, files []string, private []string) (RFD, bool) {
	rfd := RFD{Entry: name}
	readmePath := name + "/README.adoc"
	readme, err := tree.ReadFile(readmePath)
	if err != nil {
		report.problem(readmePath, "missing canonical RFD source")
		return rfd, false
	}

	var formats []checklistFormat
	for _, format := range checklistFormats {
		if slices.Contains(files, format.name) {
			formats = append(formats, format)
		}
	}
	switch len(formats) {
	case 0:
		report.problem(name, "missing implementation checklist: IMPLEMENTATION.adoc, IMPLEMENTATION.org, or IMPLEMENTATION.md")
	case 1:
		format := formats[0]
		path := name + "/" + format.name
		data, err := tree.ReadFile(path)
		if err != nil {
			report.problem(path, "cannot read implementation checklist: %v", err)
			break
		}
		checklist := string(data)
		if first, _, _ := strings.Cut(checklist, "\n"); first != format.heading(name) {
			report.problem(path, "invalid implementation checklist heading")
		}
		if !format.backlink.MatchString(checklist) {
			report.problem(path, "implementation checklist must link to its RFD")
		}
		link := regexp.MustCompile(`link:` + regexp.QuoteMeta(format.name) + `\[[^\]]+\]`)
		if !link.Match(readme) {
			report.problem(readmePath, "RFD must link to its implementation checklist")
		}
		var malformed int
		rfd.Complete, rfd.Total, malformed = countTasks(checklist)
		if malformed > 0 {
			report.problem(path, "malformed implementation task")
		}
	default:
		report.problem(name, "multiple implementation checklist formats")
	}

	if countDesignTasks(string(readme)) > 0 {
		report.problem(readmePath, "implementation checkboxes belong in a separate implementation document")
	}

	h := parseHeader(string(readme))
	for _, p := range h.problems(strings.TrimLeft(name, "0"), private) {
		report.problem(readmePath, "%s", p)
	}
	rfd.Title = h.title
	rfd.State, _ = h.attribute("state")
	rfd.Labels, _ = h.attribute("labels")
	rfd.Visibility = h.visibility(private)

	if rfd.State == "committed" {
		switch {
		case rfd.Total == 0:
			report.problem(name, "committed RFD requires a non-empty implementation checklist")
		case rfd.Complete != rfd.Total:
			report.problem(name, "committed RFD has incomplete implementation tasks (%d/%d complete)", rfd.Complete, rfd.Total)
		}
	}
	return rfd, true
}

// baseState returns the state an RFD had in base, or "" if it did not exist.
func baseState(base Tree, name string) string {
	readme, err := base.ReadFile(name + "/README.adoc")
	if err != nil {
		return ""
	}
	state, _ := parseHeader(string(readme)).attribute("state")
	return state
}

// Number returns the RFD's number.
func (r RFD) Number() int {
	n, _ := strconv.Atoi(r.Entry)
	return n
}

// checkAllLinks checks how each RFD's documents refer to other RFDs.
func checkAllLinks(report *Report, tree Tree, rfds []RFD) {
	anchors := map[string]map[string]bool{}
	for _, r := range rfds {
		readme, _ := tree.ReadFile(r.Entry + "/README.adoc")
		anchors[r.Entry] = sectionIDs(string(readme))
	}
	for _, r := range rfds {
		for _, name := range []string{"README.adoc", "IMPLEMENTATION.adoc"} {
			path := r.Entry + "/" + name
			text, err := tree.ReadFile(path)
			if err != nil {
				continue
			}
			// Report each problem once per document, with how often it occurs.
			var order []linkProblem
			counts := map[linkProblem]int{}
			for _, p := range checkLinks(string(text), r.Number(), anchors) {
				if counts[p] == 0 {
					order = append(order, p)
				}
				counts[p]++
			}
			for _, p := range order {
				message := p.message
				if counts[p] > 1 {
					message += fmt.Sprintf(" (%d times)", counts[p])
				}
				if p.fixable {
					message += "; run rfdtool link"
				}
				report.problem(path, "%s", message)
			}
		}
	}
}
