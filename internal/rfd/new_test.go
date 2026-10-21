package rfd

import (
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	f := newFixture(t)
	f.writeRFD("prediscussion", "", "adoc")

	draft := Draft{Title: "Example change", Authors: "Example Author <author@example.com>", Labels: "core, example"}
	entry, err := Create(f.dir, Template, draft)
	if err != nil {
		t.Fatal(err)
	}
	if entry != "0002" {
		t.Fatalf("created %s, want 0002", entry)
	}
	report := f.pass()
	want := RFD{Entry: "0002", Title: "Example change", State: "prediscussion", Labels: "core, example", Visibility: "public", Total: 1}
	if got := report.RFDs[1]; got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !strings.Contains(f.read("0002/IMPLEMENTATION.adoc"), "link:README.adoc[RFD 2: Example change]") {
		t.Fatal("checklist does not link to its RFD")
	}

	if _, err := Create(f.dir, Template, Draft{Title: "Two\nlines"}); err == nil {
		t.Fatal("created an RFD with a multi-line title")
	}
}
