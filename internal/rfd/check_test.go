package rfd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	t    *testing.T
	dir  string // current RFDs
	base string // base RFDs, when set
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{t: t, dir: filepath.Join(root, "rfd")}
	f.write("README.adoc", "= Test RFDs\n")
	return f
}

func (f *fixture) path(name string) string {
	return filepath.Join(f.dir, filepath.FromSlash(name))
}

func (f *fixture) write(name, text string) {
	f.t.Helper()
	path := f.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(name string) string {
	f.t.Helper()
	data, err := os.ReadFile(f.path(name))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) replace(name, old, new string) {
	f.t.Helper()
	text := f.read(name)
	if !strings.Contains(text, old) {
		f.t.Fatalf("%s does not contain %q", name, old)
	}
	f.write(name, strings.Replace(text, old, new, 1))
}

func (f *fixture) append(name, text string) {
	f.t.Helper()
	f.write(name, f.read(name)+text)
}

func (f *fixture) remove(name string) {
	f.t.Helper()
	if err := os.RemoveAll(f.path(name)); err != nil {
		f.t.Fatal(err)
	}
}

// snapshot copies the current RFDs to use as the base.
func (f *fixture) snapshot() {
	f.t.Helper()
	f.base = filepath.Join(filepath.Dir(f.dir), "base-rfd")
	if err := os.CopyFS(f.base, os.DirFS(f.dir)); err != nil {
		f.t.Fatal(err)
	}
}

// editBase rewrites a file in the base RFDs.
func (f *fixture) editBase(name, old, new string) {
	f.t.Helper()
	path := filepath.Join(f.base, filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

var checklists = map[string]string{
	"adoc": "= RFD 0001 implementation checklist\n\nImplements link:README.adoc[RFD 1: Valid RFD].\n\n* [ ] Complete the work.\n",
	"org":  "#+TITLE: RFD 0001 implementation checklist\n\nImplements [[file:README.adoc][RFD 1: Valid RFD]].\n\n- [ ] Complete the work.\n",
	"md":   "# RFD 0001 implementation checklist\n\nImplements [RFD 1: Valid RFD](README.adoc).\n\n- [ ] Complete the work.\n",
}

func (f *fixture) writeRFD(state, discussion, format string) {
	f.t.Helper()
	name := "IMPLEMENTATION." + format
	f.write("0001/README.adoc", ":authors: Example Author <author@example.com>\n"+
		":state: "+state+"\n"+
		":discussion: "+discussion+"\n"+
		":labels: software, process\n"+
		"\n= RFD 1 Valid RFD\n\n== Implementation\n\nSee link:"+name+"[implementation checklist].\n")
	f.write("0001/"+name, checklists[format])
}

// writeSecond writes RFD 2 with the given attributes after its authors and
// state.
func (f *fixture) writeSecond(attributes string) {
	f.t.Helper()
	f.write("0002/README.adoc", ":authors: Example Author <author@example.com>\n:state: prediscussion\n:discussion:\n"+
		attributes+"\n\n= RFD 2 Second RFD\n\nSee link:IMPLEMENTATION.adoc[implementation checklist].\n")
	f.write("0002/IMPLEMENTATION.adoc", "= RFD 0002 implementation checklist\n\nImplements link:README.adoc[RFD 2].\n\n* [ ] Do it.\n")
}

// writeDone writes a committed RFD with its only task done.
func (f *fixture) writeDone() {
	f.t.Helper()
	f.writeRFD("committed", "https://example.com/pull/2", "adoc")
	f.replace("0001/IMPLEMENTATION.adoc", "* [ ]", "* [X]")
}

// toOrg moves the checklist from AsciiDoc to Org.
func (f *fixture) toOrg() {
	f.t.Helper()
	text := f.read("0001/IMPLEMENTATION.adoc")
	text = strings.Replace(text, "= RFD ", "#+TITLE: RFD ", 1)
	text = strings.Replace(text, "link:README.adoc[RFD 1: Valid RFD]", "[[file:README.adoc][RFD 1: Valid RFD]]", 1)
	text = strings.Replace(text, "* [X]", "- [X]", 1)
	f.remove("0001/IMPLEMENTATION.adoc")
	f.write("0001/IMPLEMENTATION.org", text)
	f.replace("0001/README.adoc", "link:IMPLEMENTATION.adoc[", "link:IMPLEMENTATION.org[")
}

func (f *fixture) check() (*Report, error) {
	f.t.Helper()
	var base Tree
	if f.base != "" {
		base = DirTree(f.base)
	}
	return Check(f.dir, base, []string{"internal"})
}

func (f *fixture) pass() *Report {
	f.t.Helper()
	report, err := f.check()
	if err != nil {
		f.t.Fatal(err)
	}
	if len(report.Issues) > 0 {
		f.t.Fatalf("expected no issues, got %v", report.Issues)
	}
	return report
}

func (f *fixture) fail(want string) {
	f.t.Helper()
	report, err := f.check()
	if err != nil {
		if !strings.Contains(err.Error(), want) {
			f.t.Fatalf("expected %q, got error %v", want, err)
		}
		return
	}
	for _, issue := range report.Issues {
		if strings.Contains(issue.String(), want) {
			return
		}
	}
	f.t.Fatalf("expected an issue with %q, got %v", want, report.Issues)
}

func TestValid(t *testing.T) {
	for _, format := range []string{"adoc", "org", "md"} {
		t.Run(format, func(t *testing.T) {
			f := newFixture(t)
			f.writeRFD("discussion", "https://example.com/pull/1", format)
			report := f.pass()
			want := RFD{Entry: "0001", Title: "Valid RFD", State: "discussion", Labels: "software, process", Visibility: "public", Total: 1}
			if len(report.RFDs) != 1 || report.RFDs[0] != want {
				t.Fatalf("got %+v, want %+v", report.RFDs, want)
			}
		})
	}
}

func TestVisibility(t *testing.T) {
	f := newFixture(t)
	f.writeRFD("prediscussion", "", "adoc")
	f.writeSecond(":labels: internal, ops")
	report := f.pass()
	for i, want := range []string{"public", "private"} {
		if got := report.RFDs[i].Visibility; got != want {
			t.Errorf("RFD %s visibility = %s, want %s", report.RFDs[i].Entry, got, want)
		}
	}

	// Without internal among the private projects, its RFDs are public.
	report, err := Check(f.dir, nil, []string{"other"})
	if err != nil {
		t.Fatal(err)
	}
	if got := report.RFDs[1].Visibility; got != "public" {
		t.Errorf("RFD 0002 visibility = %s, want public", got)
	}
}

func TestStatusTable(t *testing.T) {
	var b bytes.Buffer
	WriteStatus(&b, []RFD{
		{Entry: "0001", Title: "Valid RFD", State: "discussion", Labels: "software, process", Complete: 1, Total: 3},
		{Entry: "0002", Title: "A title long enough to need no column", State: "prediscussion", Labels: "core"},
	}, Palette{})
	want := `RFD   State          Progress  Labels             Title
----  -------------  --------  -----------------  ------------------------------
0001  discussion     33%       software, process  Valid RFD
0002  prediscussion  -         core               A title long enough to need no column
`
	if b.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", b.String(), want)
	}
}

func TestIssues(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *fixture)
		want  string
	}{
		{"missing checklist", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.remove("0001/IMPLEMENTATION.adoc")
		}, "missing implementation checklist"},
		{"task in README", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.append("0001/README.adoc", "\n  - [ ] This belongs in the implementation document.\n")
		}, "implementation checkboxes belong in a separate implementation document"},
		{"plus task in README", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.append("0001/README.adoc", "\n+ [ ] This also belongs in the implementation document.\n")
		}, "implementation checkboxes belong in a separate implementation document"},
		{"numbered task in README", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.append("0001/README.adoc", "\n1. [X] This also belongs in the implementation document.\n")
		}, "implementation checkboxes belong in a separate implementation document"},
		{"committed with open tasks", func(f *fixture) {
			f.writeRFD("committed", "https://example.com/pull/1", "adoc")
		}, "committed RFD has incomplete implementation tasks (0/1 complete)"},
		{"committed without tasks", func(f *fixture) {
			f.writeRFD("committed", "https://example.com/pull/1", "org")
			f.replace("0001/IMPLEMENTATION.org", "- [ ] Complete the work.\n", "")
		}, "committed RFD requires a non-empty implementation checklist"},
		{"committed with only example tasks", func(f *fixture) {
			f.writeRFD("committed", "https://example.com/pull/1", "org")
			f.replace("0001/IMPLEMENTATION.org", "- [ ] Complete the work.\n", "#+begin_example\n- [X] Example only.\n#+end_example\n")
		}, "committed RFD requires a non-empty implementation checklist"},
		{"malformed task", func(f *fixture) {
			f.writeRFD("committed", "https://example.com/pull/1", "adoc")
			f.replace("0001/IMPLEMENTATION.adoc", "* [ ] Complete the work.", "* [Y] Invalid task.")
		}, "malformed implementation task"},
		{"two checklist formats", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.write("0001/IMPLEMENTATION.md", "# RFD 0001 implementation checklist\n\nImplements [RFD 1](README.adoc).\n")
		}, "multiple implementation checklist formats"},
		{"wrong checklist heading", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/IMPLEMENTATION.adoc", "= RFD 0001", "= RFD 1")
		}, "invalid implementation checklist heading"},
		{"no link to checklist", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", "link:IMPLEMENTATION.adoc[implementation checklist]", "link:IMPLEMENTATION.adoc[")
		}, "RFD must link to its implementation checklist"},
		{"no link to RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/IMPLEMENTATION.adoc", "Implements link:README.adoc[RFD 1: Valid RFD].", "Implements RFD 1.")
		}, "implementation checklist must link to its RFD"},
		{"invalid state", func(f *fixture) {
			f.writeRFD("draft", "", "adoc")
		}, "invalid state: draft"},
		{"discussion without URL", func(f *fixture) {
			f.writeRFD("discussion", "", "adoc")
		}, "state discussion requires a discussion URL"},
		{"discussion not a URL", func(f *fixture) {
			f.writeRFD("prediscussion", "#1", "adoc")
		}, "discussion must be empty or an HTTP(S) URL"},
		{"two authors attributes", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", ":state:", ":authors: Another Author <another@example.com>\n:state:")
		}, "exactly one non-empty authors attribute"},
		{"author without address", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", " <author@example.com>", "")
		}, "authors must include a name and address in angle brackets"},
		{"no labels", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", ":labels: software, process\n", "")
		}, "exactly one non-empty labels attribute"},
		{"title number", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", "= RFD 1 ", "= RFD 2 ")
		}, "title number 2 does not match directory number 1"},
		{"invalid visibility", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", ":labels:", ":visibility: internal\n:labels:")
		}, "invalid visibility: internal"},
		{"public internal RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", ":labels: software, process", ":visibility: public\n:labels: internal, process")
		}, "internal RFDs must be private"},
		{"public RFD links to private RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.writeSecond(":visibility: private\n:labels: core")
			f.append("0001/README.adoc", "\nSee link:../0002/README.adoc[RFD 2].\n")
		}, "0001/README.adoc: public RFD links to private RFD 0002"},
		{"public RFD's checklist links to private RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.writeSecond(":visibility: private\n:labels: core")
			f.append("0001/IMPLEMENTATION.adoc", "\nSee link:../0002/README.adoc[RFD 2].\n")
		}, "0001/IMPLEMENTATION.adoc: public RFD links to private RFD 0002"},
		{"unchecked task mentioning a checkbox", func(f *fixture) {
			f.writeRFD("committed", "https://example.com/pull/2", "adoc")
			f.replace("0001/IMPLEMENTATION.adoc", "* [ ] Complete the work.", "* [ ] Render [x] checkboxes.")
		}, "incomplete implementation tasks (0/1 complete)"},
		{"unpadded directory", func(f *fixture) {
			f.write("1/README.adoc", "= RFD 1 Invalid directory\n")
		}, "invalid RFD entry"},
		{"hidden directory", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.write(".draft/README.adoc", "")
		}, ".draft: invalid RFD entry"},
		{"no RFDs", func(f *fixture) {}, "no RFDs found"},
		{"backwards transition", func(f *fixture) {
			f.writeRFD("discussion", "https://example.com/pull/2", "adoc")
			f.snapshot()
			f.editBase("0001/README.adoc", ":state: discussion", ":state: published")
		}, "invalid RFD state transition from published to discussion"},
		{"deleted RFD", func(f *fixture) {
			f.writeRFD("published", "https://example.com/pull/2", "adoc")
			f.snapshot()
			f.remove("0001")
		}, "historical RFD must not be deleted"},
		{"edited final RFD", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.append("0001/README.adoc", "\nRewritten final decision.\n")
		}, "final RFD content is immutable (committed)"},
		{"edited task while moving checklist format", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.toOrg()
			f.replace("0001/IMPLEMENTATION.org", "Complete the work", "Complete the work instead")
		}, "final RFD content is immutable"},
		{"unchecked task while moving checklist format", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.toOrg()
			f.replace("0001/IMPLEMENTATION.org", "- [X]", "- [ ]")
		}, "final RFD content is immutable"},
		{"edited README while moving checklist format", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.toOrg()
			f.append("0001/README.adoc", "\nUnrelated README edit.\n")
		}, "final RFD content is immutable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)
			f.fail(tt.want)
		})
	}
}

func TestAllowed(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *fixture)
	}{
		{"committed with tasks done", func(f *fixture) {
			f.writeDone()
		}},
		{"example task in a source block", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.append("0001/README.adoc", "\n[source,text]\n----\n- [ ] Example task syntax.\n----\n")
		}},
		{"forward transition", func(f *fixture) {
			f.writeRFD("published", "https://example.com/pull/2", "adoc")
			f.snapshot()
			f.editBase("0001/README.adoc", ":state: published", ":state: discussion")
		}},
		{"final RFD checklist moved to another format", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.toOrg()
		}},
		{"final RFD relabeled", func(f *fixture) {
			f.writeDone()
			f.snapshot()
			f.replace("0001/README.adoc", ":labels: software, process", ":labels: process, software, core")
		}},
		{"private RFD links to private RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.replace("0001/README.adoc", ":labels:", ":visibility: private\n:labels:")
			f.writeSecond(":labels: internal")
			f.append("0001/README.adoc", "\nSee link:../0002/README.adoc[RFD 2].\n")
		}},
		{"public RFD links to public RFD", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.writeSecond(":labels: core")
			f.append("0001/README.adoc", "\nSee link:../0002/README.adoc[RFD 2].\n")
		}},
		{"final RFD made public", func(f *fixture) {
			f.writeDone()
			f.replace("0001/README.adoc", ":labels:", ":visibility: private\n:labels:")
			f.snapshot()
			f.replace("0001/README.adoc", ":visibility: private\n", "")
		}},
		{"index moved from Markdown to AsciiDoc", func(f *fixture) {
			f.writeRFD("prediscussion", "", "adoc")
			f.snapshot()
			if err := os.Rename(filepath.Join(f.base, "README.adoc"), filepath.Join(f.base, "README.md")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)
			f.pass()
		})
	}
}

func TestGitTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	f := newFixture(t)
	f.writeDone()
	repo := filepath.Dir(f.dir)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	rev := git("rev-parse", "HEAD")

	for _, empty := range []string{"", "0000000000000000000000000000000000000000"} {
		if base, err := NewGitTree(f.dir, empty); base != nil || err != nil {
			t.Fatalf("NewGitTree(%q) = %v, %v; want no base", empty, base, err)
		}
	}
	if _, err := NewGitTree(f.dir, "missing"); err == nil {
		t.Fatal("NewGitTree resolved a missing revision")
	}

	base, err := NewGitTree(f.dir, rev)
	if err != nil {
		t.Fatal(err)
	}
	f.append("0001/README.adoc", "\nRewritten final decision.\n")
	report, err := Check(f.dir, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Message, "final RFD content is immutable") {
		t.Fatalf("got %v", report.Issues)
	}

	f.remove("0001")
	f.write("0002/README.adoc", "")
	report, err = Check(f.dir, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(issuesText(report), "0001: historical RFD must not be deleted") {
		t.Fatalf("got %v", report.Issues)
	}
}

func issuesText(r *Report) string {
	var b strings.Builder
	for _, issue := range r.Issues {
		b.WriteString(issue.String() + "\n")
	}
	return b.String()
}
