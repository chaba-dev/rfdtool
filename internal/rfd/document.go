package rfd

import (
	"regexp"
	"slices"
	"strings"
)

// States lists the RFD lifecycle states in order.
var States = []string{"prediscussion", "ideation", "discussion", "published", "committed", "abandoned"}

// transitions lists the states each state may move to. Every state may also
// stay where it is.
var transitions = map[string][]string{
	"prediscussion": {"ideation", "discussion", "published", "committed", "abandoned"},
	"ideation":      {"prediscussion", "discussion", "published", "committed", "abandoned"},
	"discussion":    {"published", "committed", "abandoned"},
	"published":     {"committed", "abandoned"},
}

func validState(state string) bool {
	for _, s := range States {
		if s == state {
			return true
		}
	}
	return false
}

func validTransition(from, to string) bool {
	if from == to {
		return true
	}
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// final reports whether a state freezes the RFD's content.
func final(state string) bool {
	return state == "committed" || state == "abandoned"
}

// needsDiscussion reports whether a state requires a discussion URL.
func needsDiscussion(state string) bool {
	return state == "discussion" || state == "published" || state == "committed"
}

var (
	attributeLine = regexp.MustCompile(`^:([^:]+):[[:space:]]*(.*)$`)
	titleLine     = regexp.MustCompile(`^= RFD ([0-9]+) (.+)$`)
	authorAddress = regexp.MustCompile(`<[^>]+>`)
	httpURL       = regexp.MustCompile(`^https?://`)
)

// header holds the document attributes and title of an RFD's README.adoc.
type header struct {
	attributes  map[string][]string
	titles      int
	titleNumber string
	title       string
}

// parseHeader reads the attributes that precede the RFD title, and every
// title line in the document.
func parseHeader(text string) header {
	h := header{attributes: map[string][]string{}}
	for _, line := range lines(text) {
		if m := titleLine.FindStringSubmatch(line); m != nil {
			h.titles++
			if h.titles == 1 {
				h.titleNumber, h.title = m[1], m[2]
			}
			continue
		}
		if h.titles > 0 {
			continue
		}
		if m := attributeLine.FindStringSubmatch(line); m != nil {
			h.attributes[m[1]] = append(h.attributes[m[1]], m[2])
		}
	}
	return h
}

// attribute returns the first value of an attribute and how many times it
// was set.
func (h header) attribute(name string) (string, int) {
	values := h.attributes[name]
	if len(values) == 0 {
		return "", 0
	}
	return values[0], len(values)
}

// problems validates the header of the RFD numbered number. RFDs of the
// private projects must not be public.
func (h header) problems(number string, private []string) []string {
	var problems []string
	for _, name := range []string{"authors", "state", "discussion", "labels", "visibility"} {
		for _, value := range h.attributes[name] {
			if strings.Contains(value, "\t") {
				problems = append(problems, "attribute values must not contain tabs")
			}
		}
	}

	authors, n := h.attribute("authors")
	switch {
	case n != 1 || authors == "":
		problems = append(problems, "document must contain exactly one non-empty authors attribute")
	case !authorAddress.MatchString(authors):
		problems = append(problems, "authors must include a name and address in angle brackets")
	}

	state, n := h.attribute("state")
	switch {
	case n != 1 || state == "":
		problems = append(problems, "document must contain exactly one non-empty state attribute")
	case !validState(state):
		problems = append(problems, "invalid state: "+state)
	}

	discussion, n := h.attribute("discussion")
	switch {
	case n != 1:
		problems = append(problems, "document must contain exactly one discussion attribute")
	case discussion != "" && !httpURL.MatchString(discussion):
		problems = append(problems, "discussion must be empty or an HTTP(S) URL")
	case needsDiscussion(state) && discussion == "":
		problems = append(problems, "state "+state+" requires a discussion URL")
	}

	if labels, n := h.attribute("labels"); n != 1 || labels == "" {
		problems = append(problems, "document must contain exactly one non-empty labels attribute")
	}

	switch visibility, n := h.attribute("visibility"); {
	case n > 1:
		problems = append(problems, "document must contain at most one visibility attribute")
	case n == 1 && visibility != "public" && visibility != "private":
		problems = append(problems, "invalid visibility: "+visibility)
	case visibility == "public" && slices.Contains(private, h.project()):
		problems = append(problems, h.project()+" RFDs must be private")
	}

	switch {
	case h.titles != 1:
		problems = append(problems, "document must contain exactly one RFD title")
	case h.titleNumber != number:
		problems = append(problems, "title number "+h.titleNumber+" does not match directory number "+number)
	}
	return problems
}

// project returns the project an RFD belongs to: its first label.
func (h header) project() string {
	labels, _ := h.attribute("labels")
	project, _, _ := strings.Cut(labels, ",")
	return strings.TrimSpace(project)
}

// visibility returns whether an RFD is published on the public site. An
// RFD without the attribute is public, unless it belongs to one of the
// private projects.
func (h header) visibility(private []string) string {
	if visibility, _ := h.attribute("visibility"); visibility != "" {
		return visibility
	}
	if slices.Contains(private, h.project()) {
		return "private"
	}
	return "public"
}

// lines splits text into lines without a trailing empty line.
func lines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
