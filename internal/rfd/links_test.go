package rfd

import (
	"strings"
	"testing"
)

func TestLinkMentions(t *testing.T) {
	exists := func(n int) bool { return n >= 1 && n <= 20 }
	tests := []struct {
		name, text, want string
	}{
		{"single", "See RFD 5.", "See link:../0005/README.adoc[RFD 5]."},
		{"list", "RFDs 12, 13, and 14 apply.",
			"RFDs link:../0012/README.adoc[12], link:../0013/README.adoc[13], and link:../0014/README.adoc[14] apply."},
		{"range", "RFDs 1 to 7", "RFDs link:../0001/README.adoc[1] to link:../0007/README.adoc[7]"},
		{"across lines", "as RFD\n5 says", "as link:../0005/README.adoc[RFD 5] says"},
		{"self", "This is RFD 3.", "This is RFD 3."},
		{"missing", "RFD 99 and RFD 2", "RFD 99 and link:../0002/README.adoc[RFD 2]"},
		{"already linked", "link:../0005/README.adoc[RFD 5]", "link:../0005/README.adoc[RFD 5]"},
		{"url text", "https://example.com/rfd[RFD 5]", "https://example.com/rfd[RFD 5]"},
		{"literal", "Use `RFD 5` and +RFD 6+.", "Use `RFD 5` and +RFD 6+."},
		{"title", "= RFD 5 Title\n", "= RFD 5 Title\n"},
		{"comment", "// RFD 5\n", "// RFD 5\n"},
		{"listing", "----\nRFD 5\n----\nRFD 5\n", "----\nRFD 5\n----\nlink:../0005/README.adoc[RFD 5]\n"},
		{"word boundary", "RFD 5s and XRFD 5", "RFD 5s and XRFD 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LinkMentions(tt.text, 3, exists); got != tt.want {
				t.Errorf("LinkMentions(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestCheckLinks(t *testing.T) {
	anchors := map[string]map[string]bool{
		"0001": {"goal": true},
		"0002": {"design": true},
	}
	tests := []struct {
		name, text, want string
	}{
		{"linked", "See link:../0002/README.adoc[RFD 2] and link:../0002/README.adoc#design[its design].", ""},
		{"own section", "See <<goal>> and <<goal,the goal>>.", ""},
		{"unlinked", "See RFD 2.", "RFD 2 is mentioned without a link"},
		{"mentions missing", "See RFD 9.", "mentions RFD 9, which does not exist"},
		{"links missing", "See link:../0009/README.adoc[RFD 9].", "links to RFD 9, which does not exist"},
		{"missing anchor", "See link:../0002/README.adoc#goal[goal].", "links to #goal, which RFD 2 does not have"},
		{"wrong label", "See link:../0002/README.adoc[RFD 3].", `link to RFD 2 is labeled "RFD 3"`},
		{"missing xref", "See <<_goal>>.", "refers to <<_goal>>, which this RFD does not have"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, p := range checkLinks(tt.text, 1, anchors) {
				got = append(got, p.message)
			}
			if tt.want == "" && len(got) > 0 || tt.want != "" && !strings.Contains(strings.Join(got, "\n"), tt.want) {
				t.Errorf("checkLinks(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestSectionIDs(t *testing.T) {
	ids := sectionIDs("= RFD 1 Title\n\n== Goal and scope\n\n=== `rfd check` v2.0\n\n[[custom]]\n[#other]\n----\n== Not a section\n----\n")
	for _, id := range []string{"goal-and-scope", "rfd-check-v2-0", "custom", "other"} {
		if !ids[id] {
			t.Errorf("missing id %q in %v", id, ids)
		}
	}
	if ids["not-a-section"] {
		t.Errorf("listing text became a section: %v", ids)
	}
}

func TestWrapChanged(t *testing.T) {
	before := "Intro.\nRFD 7 and RFD 8 shipped parts of sync before this RFD was settled. This RFD\n"
	after := LinkMentions(before, 1, func(int) bool { return true })
	got := wrapChanged(before, after, 80)
	want := "Intro.\nlink:../0007/README.adoc[RFD 7] and link:../0008/README.adoc[RFD 8] shipped\nparts of sync before this RFD was settled. This RFD\n"
	if got != want {
		t.Errorf("wrapChanged = %q, want %q", got, want)
	}

	item := "* " + strings.Repeat("word ", 14) + "RFD 2 here"
	got = wrapChanged(item, LinkMentions(item, 1, func(int) bool { return true }), 80)
	if lines := strings.Split(got, "\n"); len(lines) != 2 || !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("list item wrapped as %q", got)
	}
}

func TestLinksInCheck(t *testing.T) {
	f := newFixture(t)
	f.writeRFD("prediscussion", "", "adoc")
	f.writeSecond(":labels: core")
	f.append("0001/README.adoc", "\nRFD 2 applies.\n")
	f.fail("RFD 2 is mentioned without a link; run rfdtool link")

	changed, err := LinkAll(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || !strings.HasSuffix(changed[0], "0001/README.adoc") {
		t.Fatalf("LinkAll changed %v", changed)
	}
	if !strings.Contains(f.read("0001/README.adoc"), "link:../0002/README.adoc[RFD 2] applies.") {
		t.Fatalf("README not linked:\n%s", f.read("0001/README.adoc"))
	}
	f.pass()
}

func TestLinkingFinalRFD(t *testing.T) {
	f := newFixture(t)
	f.writeDone()
	f.writeSecond(":labels: core")
	f.append("0001/README.adoc", "\nThis RFD builds on the ideas in the earlier design work, as RFD 2 describes.\n")
	f.snapshot()
	if _, err := LinkAll(f.dir); err != nil {
		t.Fatal(err)
	}
	f.pass()
}
