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
		{"single", "See RFD 5.", "See xref:../0005/README.adoc[RFD 5]."},
		{"list", "RFDs 12, 13, and 14 apply.",
			"RFDs xref:../0012/README.adoc[12], xref:../0013/README.adoc[13], and xref:../0014/README.adoc[14] apply."},
		{"range", "RFDs 1 to 7", "RFDs xref:../0001/README.adoc[1] to xref:../0007/README.adoc[7]"},
		{"across lines", "as RFD\n5 says", "as xref:../0005/README.adoc[RFD 5] says"},
		{"self", "This is RFD 3.", "This is RFD 3."},
		{"missing", "RFD 99 and RFD 2", "RFD 99 and xref:../0002/README.adoc[RFD 2]"},
		{"already linked", "xref:../0005/README.adoc[RFD 5]", "xref:../0005/README.adoc[RFD 5]"},
		{"url text", "https://example.com/rfd[RFD 5]", "https://example.com/rfd[RFD 5]"},
		{"literal", "Use `RFD 5` and +RFD 6+.", "Use `RFD 5` and +RFD 6+."},
		{"title", "= RFD 5 Title\n", "= RFD 5 Title\n"},
		{"comment", "// RFD 5\n", "// RFD 5\n"},
		{"listing", "----\nRFD 5\n----\nRFD 5\n", "----\nRFD 5\n----\nxref:../0005/README.adoc[RFD 5]\n"},
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
		"0001": {"_goal": true},
		"0002": {"_design": true},
	}
	tests := []struct {
		name, text, want string
	}{
		{"linked", "See xref:../0002/README.adoc[RFD 2] and xref:../0002/README.adoc#_design[its design].", ""},
		{"own section", "See <<_goal>> and <<_goal,the goal>>.", ""},
		{"unlinked", "See RFD 2.", "RFD 2 is mentioned without a link"},
		{"mentions missing", "See RFD 9.", "mentions RFD 9, which does not exist"},
		{"links missing", "See xref:../0009/README.adoc[RFD 9].", "links to RFD 9, which does not exist"},
		{"missing anchor", "See xref:../0002/README.adoc#_goal[goal].", "links to #_goal, which RFD 2 does not have"},
		{"id without the default prefix", "See xref:../0002/README.adoc#design[design].", "links to #design, which RFD 2 does not have"},
		{"link macro", "See link:../0002/README.adoc[RFD 2].", "links to RFD 2 with link:; use xref:"},
		{"wrong label", "See xref:../0002/README.adoc[RFD 3].", `link to RFD 2 is labeled "RFD 3"`},
		{"missing xref", "See <<goal>>.", "refers to <<goal>>, which this RFD does not have"},
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
	for _, id := range []string{"_goal_and_scope", "_rfd_check_v2_0", "custom", "other"} {
		if !ids[id] {
			t.Errorf("missing id %q in %v", id, ids)
		}
	}
	if ids["_not_a_section"] {
		t.Errorf("listing text became a section: %v", ids)
	}

	// Ids as asciidoctor.js gives them with its default settings.
	for _, tt := range []struct {
		name, text string
		want, not  []string
	}{
		{"anchor replaces title id", "[[values]]\n=== Values and drift\n", []string{"values"}, []string{"_values_and_drift"}},
		{"anchor across attributes", "[#ext]\n[role=wide]\n== Extension points\n", []string{"ext"}, []string{"_extension_points"}},
		{"anchor across a blank line", "[[a]]\n\n== Blank between\n", []string{"a"}, []string{"_blank_between"}},
		{"anchor on a paragraph", "[[p]]\nA paragraph.\n\n== Para before\n", []string{"p", "_para_before"}, nil},
		{"anchor in the title", "== Title anchor [[ta]]\n", []string{"ta"}, []string{"_title_anchor"}},
		{"id shorthand after a style", "[discrete#dd.role]\n== Discrete\n", []string{"dd"}, []string{"_discrete"}},
		{"id attribute", "[id=\"sec\"]\n== Id attr\n", []string{"sec"}, []string{"_id_attr"}},
		{"anchor in a quoted value", "[role=\"[[x]]\"]\n== Title\n", []string{"_title"}, nil},
		{"anchor in monospace", "Use `[[x]]value` here.\n", []string{"x"}, nil},
		{"TOML in a listing", "[source,toml]\n----\n[[exporters]]\n----\n\n== After\n", []string{"_after"}, []string{"exporters"}},
		{"listing with macros", "[subs=\"+macros\"]\n----\n[[x]]\n----\n\n== After\n", []string{"x", "_after"}, nil},
		{"separators", "== .NET & the a__b - c. \n", []string{"_net_the_a_b_c"}, nil},
		{"positional with a space", "[foo #bar]\n== Title\n", []string{"_title"}, nil},
	} {
		ids := sectionIDs("= RFD 1 Title\n\n" + tt.text)
		for _, id := range tt.want {
			if !ids[id] {
				t.Errorf("%s: missing id %q in %v", tt.name, id, ids)
			}
		}
		for _, id := range tt.not {
			if ids[id] {
				t.Errorf("%s: unexpected id %q in %v", tt.name, id, ids)
			}
		}
	}
}

func TestWrapChanged(t *testing.T) {
	before := "Intro.\nRFD 7 and RFD 8 shipped parts of sync before this RFD was settled. This RFD\n"
	after := LinkMentions(before, 1, func(int) bool { return true })
	got := wrapChanged(before, after, 80)
	want := "Intro.\nxref:../0007/README.adoc[RFD 7] and xref:../0008/README.adoc[RFD 8] shipped\nparts of sync before this RFD was settled. This RFD\n"
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
	if !strings.Contains(f.read("0001/README.adoc"), "xref:../0002/README.adoc[RFD 2] applies.") {
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
