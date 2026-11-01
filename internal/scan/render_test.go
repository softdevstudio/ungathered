package scan

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func sample(t *testing.T) Report {
	t.Helper()
	root, cfg := t.TempDir(), t.TempDir()
	put(t, root, "articles/cafe.md", "no credit")
	put(t, root, "articles/a,b%.md", "Photo credit: x")
	put(t, cfg, "slugs.txt", "cafe\na,b%\nmarket\n")
	m := manifest(t, `{"version":1,"collections":[{"name":"posts","items":"articles/*.md","kind":"file","roster":{"file":"slugs.txt","match":"stem"},"require":[{"name":"image","path":"images/{stem}.jpg"},{"name":"credit","path":"{item}","contains":"Photo credit: .+"}]}]}`)
	r, e := Scan(root, m, Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestGitHubAnnotations(t *testing.T) {
	r := sample(t)
	var b bytes.Buffer
	ws := filepath.Dir(r.Root)
	if e := renderGitHub(&b, r, ws); e != nil {
		t.Fatal(e)
	}
	out := b.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// totals line, one missing_item, two missing images, one content mismatch
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "UnGathered ") {
		t.Fatal(out)
	}
	rel := filepath.Base(r.Root)
	for _, want := range []string{
		"::error title=UnGathered missing_item::posts: market (",
		"::error file=" + rel + "/articles/a%2Cb%25.md,title=UnGathered missing::posts / articles/a,b%25.md / image: images/a,b%25.jpg",
		"::error file=" + rel + "/articles/cafe.md,title=UnGathered content_mismatch::",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if ghProp("a:b,c%\n") != "a%3Ab%2Cc%25%0A" || ghData("a:b,c%\r") != "a:b,c%25%0D" {
		t.Fatal("escaping")
	}
	// Outside the workspace, no file property.
	b.Reset()
	if e := renderGitHub(&b, r, t.TempDir()); e != nil || strings.Contains(b.String(), "file=") {
		t.Fatal(b.String(), e)
	}
	// Newlines in data cannot start a second command.
	b.Reset()
	r.Issues = []Issue{{Collection: "c", Path: "x\n::error::forged", Status: "scan_error", Detail: "d"}}
	r.Collections = nil
	_ = renderGitHub(&b, r, "")
	if strings.Count(b.String(), "\n::") != 1 {
		t.Fatal(b.String())
	}
}

func TestMarkdownSummary(t *testing.T) {
	r := sample(t)
	var b bytes.Buffer
	if e := Render(&b, r, "markdown"); e != nil {
		t.Fatal(e)
	}
	out := b.String()
	for _, want := range []string{"### UnGathered", ": incomplete", "| 2 | 0 | 4 | 1 | 4 | 0 |", "Roster slugs.txt for posts: 3 entries, 2 found, 1 missing, 0 unlisted.", "| missing_item | posts |", `images/a,b%.jpg`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	r.Issues = nil
	for i := 0; i < MaxSummaryRows+5; i++ {
		r.Issues = append(r.Issues, Issue{Collection: "c|<b>", Path: "p", Status: "missing_item", Detail: "d\nx"})
	}
	b.Reset()
	_ = Render(&b, r, "markdown")
	if !strings.Contains(b.String(), "more problems are in the full report") || strings.Contains(b.String(), "<b>") || strings.Contains(b.String(), "c|") {
		t.Fatal(b.String()[:400])
	}
	complete := Report{ToolVersion: "t", Status: "complete"}
	b.Reset()
	_ = Render(&b, complete, "markdown")
	if !strings.Contains(b.String(), "Every selected item has its declared companions.") {
		t.Fatal(b.String())
	}
}

func TestTextAndHTMLShowRoster(t *testing.T) {
	r := sample(t)
	for _, f := range []string{"text", "html"} {
		var b bytes.Buffer
		if e := Render(&b, r, f); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(b.String(), "3 entries, 2 found, 1 missing") || !strings.Contains(b.String(), "market") {
			t.Errorf("%s: %s", f, b.String())
		}
	}
}

func TestTextFormatCannotStartALogCommand(t *testing.T) {
	r := Report{ToolVersion: "t", Status: "incomplete", Issues: []Issue{{Collection: "c", Path: "x\n::add-mask::secret\r|y.md", Status: "missing_item", Detail: "d\x1b[31m"}}}
	var b bytes.Buffer
	if e := Render(&b, r, "text"); e != nil {
		t.Fatal(e)
	}
	for _, line := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(line, "::") || strings.ContainsAny(line, "\r\x1b") {
			t.Fatalf("%q", b.String())
		}
	}
	if !strings.Contains(b.String(), `x\x0a::add-mask::secret\x0d|y.md`) {
		t.Fatal(b.String())
	}
}

func TestMarkdownStaysSmall(t *testing.T) {
	r := Report{ToolVersion: "t", Status: "incomplete"}
	long := strings.Repeat("p", 20000)
	for i := 0; i < MaxSummaryRows; i++ {
		r.Issues = append(r.Issues, Issue{Collection: long, Path: long, Status: "missing_item", Detail: long})
	}
	var b bytes.Buffer
	_ = Render(&b, r, "markdown")
	// Cells are cut at MaxSummaryCell, so 200 rows of huge paths stay far
	// below GitHub's 1 MiB summary limit.
	if b.Len() > 1<<20 || strings.Contains(b.String(), strings.Repeat("p", MaxSummaryCell+1)) {
		t.Fatal(b.Len())
	}
}
