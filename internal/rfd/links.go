package rfd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// An RFD that mentions another RFD links to it with an AsciiDoc cross
// reference, so readers can follow the reference on the site and on GitHub:
//
//	xref:../0005/README.adoc[RFD 5]
//	RFDs xref:../0013/README.adoc[13] and xref:../0014/README.adoc[14]
//	xref:../0005/README.adoc#_goal[its goal]
//
// Asciidoctor renders these as links to README.html, which a site rewrites
// into page links. Section ids are Asciidoctor's defaults, such as _goal.

var (
	// mention matches "RFD 5", and "RFDs 12, 13, and 14" or "RFDs 1 to 7"
	// with the numbers in group 2.
	mention = regexp.MustCompile(`\b(RFDs?)[ \t\n]+([0-9]+(?:(?:,[ \t\n]*(?:(?:and|or)[ \t\n]+)?|[ \t\n]+(?:and|or|to|through)[ \t\n]+)[0-9]+)*)\b`)
	number  = regexp.MustCompile(`[0-9]+`)

	// rfdLinkMacro matches a link to another RFD, with the macro in group 1,
	// the RFD number in group 2, the anchor in group 3, and the link text in
	// group 4. Only xref is valid; link is matched to reject it.
	rfdLinkMacro = regexp.MustCompile(`(xref|link):\.\./([0-9]{4})/README\.adoc(?:#([^\[\s]*))?\[([^\]]*)\]`)
	// localXref matches <<anchor>> and <<anchor,text>>.
	localXref = regexp.MustCompile(`<<([^,>]+)(?:,[^>]*)?>>`)

	// Spans where a mention is not prose: link and xref macros and URLs with
	// their text, cross references, and inline literals.
	skipSpan = regexp.MustCompile(`(?:link:|xref:|https?://)[^\[\s]*\[[^\]]*\]|<<[^>]*>>|` + "`[^`\n]*`" + `|\+[^+\n]*\+`)
	// Lines where a mention is not prose: titles, attributes, and comments.
	skipLine = regexp.MustCompile(`^(?:= |:[^:]+:|//)`)
	// Delimiters of blocks whose text is not prose: listings, literals,
	// comments, and passthroughs.
	literalDelimiter = regexp.MustCompile(`^(?:----|\.\.\.\.|////|\+\+\+\+)[ \t]*$`)

	explicitAnchor = regexp.MustCompile(`\[\[([^\],]+)(?:,[^\]]*)?\]\]|\[#([^\].,]+)[\].,]|\[id=["']?([^\],"']+)`)
	sectionTitle   = regexp.MustCompile(`^=={1,5} +(.+?)[ \t]*$`)
)

// prose returns a copy of text with everything that is not prose, as in
// skipSpan, skipLine, and literalDelimiter, replaced by spaces, so offsets in
// it are offsets in text.
func prose(text string) string {
	out := []byte(text)
	blank := func(start, end int) {
		for i := start; i < end; i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	offset, literal := 0, ""
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case literal != "":
			blank(offset, offset+len(line))
			if trimmed == literal {
				literal = ""
			}
		case literalDelimiter.MatchString(trimmed):
			blank(offset, offset+len(line))
			literal = strings.TrimSpace(trimmed)
		case skipLine.MatchString(trimmed):
			blank(offset, offset+len(line))
		default:
			for _, span := range skipSpan.FindAllStringIndex(line, -1) {
				blank(offset+span[0], offset+span[1])
			}
		}
		offset += len(line)
	}
	return string(out)
}

// A linkProblem is a problem with how a document refers to other RFDs.
type linkProblem struct {
	message string
	fixable bool // LinkMentions fixes it
}

// checkLinks finds unlinked mentions of other RFDs, links to RFDs that do not
// exist or that name another RFD, and anchors that do not exist, in the
// document text of the RFD numbered self. anchors maps each RFD entry to the
// section ids in its README.
func checkLinks(text string, self int, anchors map[string]map[string]bool) []linkProblem {
	var problems []linkProblem
	plain := prose(text)
	for _, m := range mention.FindAllStringSubmatchIndex(plain, -1) {
		for _, n := range number.FindAllString(plain[m[4]:m[5]], -1) {
			num, _ := strconv.Atoi(n)
			if num == self {
				continue
			}
			if _, ok := anchors[fmt.Sprintf("%04d", num)]; !ok {
				problems = append(problems, linkProblem{message: "mentions RFD " + n + ", which does not exist"})
				continue
			}
			problems = append(problems, linkProblem{message: "RFD " + n + " is mentioned without a link", fixable: true})
		}
	}

	for _, m := range rfdLinkMacro.FindAllStringSubmatch(text, -1) {
		macro, target, anchor, label := m[1], m[2], m[3], m[4]
		ids, ok := anchors[target]
		switch {
		case macro == "link":
			problems = append(problems, linkProblem{message: "links to RFD " + strings.TrimLeft(target, "0") + " with link:; use xref:"})
			continue
		case !ok:
			problems = append(problems, linkProblem{message: "links to RFD " + strings.TrimLeft(target, "0") + ", which does not exist"})
			continue
		case anchor != "" && !ids[anchor]:
			problems = append(problems, linkProblem{message: "links to #" + anchor + ", which RFD " + strings.TrimLeft(target, "0") + " does not have"})
		}
		if named := mention.FindStringSubmatch(label); named != nil {
			want := strings.TrimLeft(target, "0")
			if !slices.Contains(number.FindAllString(named[2], -1), want) {
				problems = append(problems, linkProblem{message: fmt.Sprintf("link to RFD %s is labeled %q", want, label)})
			}
		}
	}

	own := anchors[fmt.Sprintf("%04d", self)]
	for _, m := range localXref.FindAllStringSubmatch(text, -1) {
		if id := strings.TrimSpace(m[1]); !own[id] && !strings.Contains(id, "#") && !strings.Contains(id, ".adoc") {
			problems = append(problems, linkProblem{message: "refers to <<" + id + ">>, which this RFD does not have"})
		}
	}
	return problems
}

// LinkMentions rewrites unlinked mentions of other existing RFDs in text into
// links. exists reports whether an RFD number exists.
func LinkMentions(text string, self int, exists func(int) bool) string {
	plain := prose(text)
	var b strings.Builder
	last := 0
	for _, m := range mention.FindAllStringSubmatchIndex(plain, -1) {
		word, numbers := text[m[2]:m[3]], text[m[4]:m[5]]
		var linked strings.Builder
		changed, prev := false, 0
		for _, n := range number.FindAllStringIndex(numbers, -1) {
			num, _ := strconv.Atoi(numbers[n[0]:n[1]])
			linked.WriteString(numbers[prev:n[0]])
			prev = n[1]
			if num == self || !exists(num) {
				linked.WriteString(numbers[n[0]:n[1]])
				continue
			}
			label := numbers[n[0]:n[1]]
			if word == "RFD" {
				label = "RFD " + label
			}
			fmt.Fprintf(&linked, "xref:../%04d/README.adoc[%s]", num, label)
			changed = true
		}
		if !changed {
			continue
		}
		linked.WriteString(numbers[prev:])
		b.WriteString(text[last:m[0]])
		if word == "RFDs" {
			b.WriteString("RFDs" + text[m[3]:m[4]] + linked.String())
		} else {
			b.WriteString(linked.String())
		}
		last = m[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// sectionIDs returns the ids a document's sections and anchors get, with
// Asciidoctor's default settings: "_" before and between words. A section whose
// block attributes or title set an id gets that id instead of one from its
// title. Anchors in literal blocks, such as TOML tables in a listing, are
// text, unless the block turns substitutions on. When unsure, it keeps an id:
// accepting a link to a missing section is better than rejecting a valid one.
func sectionIDs(text string) map[string]bool {
	ids := map[string]bool{}
	lines := strings.Split(text, "\n")
	plainLines := strings.Split(prose(text), "\n")
	literal := literalLines(lines)
	anchored := false // the block attributes above this line set an id
	for i, line := range lines {
		if literal[i] {
			anchored = false
			continue
		}
		anchors := explicitAnchor.FindAllStringSubmatch(line, -1)
		for _, m := range anchors {
			for _, id := range m[1:] {
				if id != "" {
					ids[strings.TrimSpace(id)] = true
				}
			}
		}
		switch m := sectionTitle.FindStringSubmatch(line); {
		case m != nil:
			if strings.TrimSpace(plainLines[i]) != "" && !anchored && len(anchors) == 0 {
				ids[sectionID(m[1])] = true
			}
			anchored = false
		case blockAttributes.MatchString(line):
			if id, ok := blockID(line); ok {
				anchored = true
				if id != "" {
					ids[id] = true
				}
			}
		case strings.TrimSpace(line) != "":
			// Block attributes apply across blank lines, to the next block.
			anchored = false
		}
	}
	return ids
}

var (
	// blockAttributes matches a line of block attributes, such as [[id]],
	// [#id.role], or [source,toml], which apply to the block or section below.
	blockAttributes = regexp.MustCompile(`^\[[^\]].*\][ \t]*$`)
	// anchorLine matches a line of block anchors, such as [[id]] or
	// [[id,text]][[other]].
	anchorLine = regexp.MustCompile(`^(?:\[\[[^\[\]]+\]\])+[ \t]*$`)
	// substitutions matches a block attribute that sets substitutions.
	substitutions = regexp.MustCompile(`\bsubs=`)
)

// blockID reports whether a block attribute line gives the block below an
// id: [[id]], an id shorthand in the first attribute as in [#id] or
// [source#id], or id=. It returns the id, except for [[id]], whose ids
// explicitAnchor finds. Text that only looks like an anchor inside a quoted
// value, as in [role="[[x]]"], sets no id.
func blockID(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if anchorLine.MatchString(line) {
		return "", true
	}
	if strings.HasPrefix(line, "[[") {
		return "", false
	}
	for i, attr := range splitAttributes(line[1 : len(line)-1]) {
		attr = strings.TrimSpace(attr)
		if id, ok := strings.CutPrefix(attr, "id="); ok && strings.Trim(id, `"'`) != "" {
			return strings.Trim(id, `"'`), true
		}
		// Asciidoctor reads no shorthand from a value with spaces.
		if i == 0 && !strings.ContainsAny(attr, "=\"' \t") {
			if _, id, ok := strings.Cut(attr, "#"); ok {
				// Roles (.) and options (%) may follow the id.
				if i := strings.IndexAny(id, ".%"); i >= 0 {
					id = id[:i]
				}
				if id != "" {
					return id, true
				}
			}
		}
	}
	return "", false
}

// splitAttributes splits a block attribute list at the commas outside quotes.
func splitAttributes(list string) []string {
	var out []string
	start, quote := 0, rune(0)
	for i, r := range list {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ',':
			out = append(out, list[start:i])
			start = i + 1
		}
	}
	return append(out, list[start:])
}

// literalLines reports which lines are in literal blocks or their
// delimiters: listings, literals, comments, and passthroughs. Blocks whose
// attributes set substitutions, which can turn macros such as anchors on, are
// not literal here, except comments, which never render.
func literalLines(lines []string) []bool {
	literal := make([]bool, len(lines))
	open, skip, attrs := "", false, ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case open != "":
			literal[i] = skip
			if trimmed == open {
				open = ""
			}
		case literalDelimiter.MatchString(line):
			open = trimmed
			skip = trimmed == "////" || !substitutions.MatchString(attrs)
			literal[i] = skip
		}
		if blockAttributes.MatchString(line) && open == "" {
			attrs += line
		} else if trimmed != "" {
			attrs = ""
		}
	}
	return literal
}

// sectionID mirrors the section ids asciidoctor.js generates with its default
// idprefix and idseparator, both "_": it drops other characters, turns
// spaces, dots, and hyphens into "_", collapses each run of "_" into one, and
// drops a trailing one.
func sectionID(title string) string {
	title = regexp.MustCompile(`<[^>]+>|&[a-z][a-z]+\d{0,2};|&#\d+;|&#x[0-9a-f]+;`).ReplaceAllString(strings.ToLower(title), "")
	var b strings.Builder
	b.WriteByte('_')
	for _, r := range title {
		switch {
		case r == ' ' || r == '.' || r == '-':
			b.WriteByte('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(r)
		}
	}
	id := regexp.MustCompile(`_+`).ReplaceAllString(b.String(), "_")
	if len(id) > 1 {
		id = strings.TrimSuffix(id, "_")
	}
	return id
}

// unlinkRFDs replaces links to other RFDs with their text, and drops the
// target of cross references, so that adding or fixing a link is not a change
// to a final RFD's decision.
func unlinkRFDs(line string) string {
	return xrefTarget.ReplaceAllString(rfdLinkMacro.ReplaceAllString(line, "$4"), "<<")
}

var xrefTarget = regexp.MustCompile(`<<[^,>]+`)

// LinkAll links unlinked mentions of other RFDs in every RFD's README and
// AsciiDoc checklist under dir, and returns the files it changed.
func LinkAll(dir string) ([]string, error) {
	names, err := DirTree(dir).ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("RFD directory not found: %s", dir)
	}
	exists := map[int]bool{}
	for _, name := range names {
		if entryName.MatchString(name) {
			n, _ := strconv.Atoi(name)
			exists[n] = true
		}
	}
	var changed []string
	for _, name := range names {
		if !entryName.MatchString(name) {
			continue
		}
		self, _ := strconv.Atoi(name)
		for _, file := range []string{"README.adoc", "IMPLEMENTATION.adoc"} {
			path := filepath.Join(dir, name, file)
			text, err := os.ReadFile(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return changed, err
			}
			linked := wrapChanged(string(text), LinkMentions(string(text), self, func(n int) bool { return exists[n] }), 80)
			if linked == string(text) {
				continue
			}
			if err := os.WriteFile(path, []byte(linked), 0o644); err != nil {
				return changed, err
			}
			changed = append(changed, path)
		}
	}
	return changed, nil
}

var (
	listMarker = regexp.MustCompile(`^[ \t]*(?:[*.-]+|[0-9]+\.)[ \t]+(?:\[[ xX]\][ \t]+)?`)
	noWrap     = regexp.MustCompile(`^(?:=|\||:|//|\[)`)
)

// wrapChanged wraps the lines of after that are not in before and are longer
// than width, so linking a mention keeps source lines short. Lines that would
// change meaning when split, such as titles and table rows, stay as they are.
func wrapChanged(before, after string, width int) string {
	old := map[string]bool{}
	for _, line := range strings.Split(before, "\n") {
		old[line] = true
	}
	plainLines := strings.Split(prose(after), "\n")
	var out []string
	for i, line := range strings.Split(after, "\n") {
		if old[line] || len(line) <= width || noWrap.MatchString(line) || strings.TrimSpace(plainLines[i]) == "" {
			out = append(out, line)
			continue
		}
		out = append(out, wrapLine(line, width)...)
	}
	return strings.Join(out, "\n")
}

// wrapLine splits line at spaces outside links and literals, indenting the
// continuation lines of a list item under its text.
func wrapLine(line string, width int) []string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	if m := listMarker.FindString(line); m != "" {
		indent = strings.Repeat(" ", len(m))
	}
	plain := []byte(line)
	for _, span := range skipSpan.FindAllStringIndex(line, -1) {
		for i := span[0]; i < span[1]; i++ {
			plain[i] = 'x'
		}
	}
	var out []string
	start, prefix := 0, ""
	for len(prefix)+len(line)-start > width {
		cut := -1
		for i := start; i < len(line) && len(prefix)+i-start <= width; i++ {
			if plain[i] == ' ' && i > start+len(listMarker.FindString(line[start:])) {
				cut = i
			}
		}
		if cut < 0 {
			break
		}
		out = append(out, prefix+line[start:cut])
		start, prefix = cut+1, indent
	}
	return append(out, prefix+line[start:])
}
