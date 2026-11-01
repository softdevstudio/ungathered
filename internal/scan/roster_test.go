package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rosterManifest(roster string) string {
	return strings.Replace(docs, `"kind":"directory"`, `"kind":"directory","roster":`+roster, 1)
}

func TestRosterTextFindsMissingItems(t *testing.T) {
	root, cfg := t.TempDir(), t.TempDir()
	put(t, root, "products/alpha/spec.txt", "x")
	put(t, root, "products/alpha/drawing.txt", "x")
	put(t, root, "products/gamma/spec.txt", "x")
	put(t, root, "products/gamma/drawing.txt", "x")
	put(t, cfg, "skus.txt", "\xef\xbb\xbf# active products\r\nalpha\r\n\r\n  beta  \r\ndelta\n")
	m := manifest(t, rosterManifest(`{"file":"skus.txt"}`))
	r, e := Scan(root, m, Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil {
		t.Fatal(e)
	}
	ro := r.Collections[0].Roster
	if ro == nil || ro.Entries != 3 || ro.Found != 1 || ro.Missing != 2 || ro.Unlisted != 0 || ro.Match != "name" || len(ro.SHA256) != 64 {
		t.Fatalf("%+v", ro)
	}
	if r.Status != "incomplete" || r.Totals.Gaps != 2 || r.Totals.Errors != 0 || r.Totals.CompleteItems != 2 {
		t.Fatalf("%+v", r.Totals)
	}
	if r.Issues[0].Status != "missing_item" || r.Issues[0].Path != "beta" || r.Issues[1].Path != "delta" {
		t.Fatalf("%+v", r.Issues)
	}
	// gamma is not listed; ignored by default, a gap on request.
	m = manifest(t, rosterManifest(`{"file":"skus.txt","unlisted":"gap"}`))
	r, e = Scan(root, m, Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil || r.Totals.Gaps != 3 || r.Issues[2].Status != "unlisted_item" || r.Issues[2].Path != "products/gamma" || r.Collections[0].Roster.Unlisted != 1 {
		t.Fatalf("%+v %v", r.Issues, e)
	}
	// Creating the missing folders completes it.
	for _, n := range []string{"beta", "delta"} {
		put(t, root, "products/"+n+"/spec.txt", "x")
		put(t, root, "products/"+n+"/drawing.txt", "x")
	}
	r, e = Scan(root, manifest(t, rosterManifest(`{"file":"skus.txt"}`)), Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil || r.Status != "complete" || r.Collections[0].Roster.Found != 3 {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestRosterCSVAndMatchKeys(t *testing.T) {
	root, cfg := t.TempDir(), t.TempDir()
	put(t, root, "articles/2026/cafe.md", "x")
	put(t, root, "articles/2025/harbor.md", "x")
	put(t, cfg, "plan.csv", "id, slug ,owner\n1,cafe,ann\n2,\"harbor\",bo\n3,market,cy\n")
	put(t, cfg, "paths.txt", "articles/2026/cafe.md\narticles/2024/old.md\n")
	base := `{"version":1,"collections":[{"name":"posts","items":"articles/**/*.md","kind":"file","roster":%s,"require":[{"name":"self","path":"{item}"}]}]}`
	r, e := Scan(root, manifest(t, fmt.Sprintf(base, `{"file":"plan.csv","column":"slug","match":"stem"}`)), Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil || r.Totals.Gaps != 1 || r.Issues[0].Path != "market" {
		t.Fatalf("%+v %v", r.Issues, e)
	}
	r, e = Scan(root, manifest(t, fmt.Sprintf(base, `{"file":"paths.txt","match":"item","unlisted":"gap"}`)), Options{MaxContentBytes: 1024, RosterDir: cfg})
	if e != nil || r.Totals.Gaps != 2 || r.Issues[0].Path != "articles/2024/old.md" || r.Issues[1].Path != "articles/2025/harbor.md" || r.Issues[1].Status != "unlisted_item" {
		t.Fatalf("%+v %v", r.Issues, e)
	}
	// Absolute roster paths work without a roster folder.
	abs := filepath.ToSlash(filepath.Join(cfg, "paths.txt"))
	if _, e = Scan(root, manifest(t, fmt.Sprintf(base, `{"file":"`+abs+`","match":"item"}`)), Options{MaxContentBytes: 1024}); e != nil && !strings.Contains(abs, ":") {
		t.Fatal(e)
	}
}

func TestRosterFailuresAreInvalidInput(t *testing.T) {
	root, cfg := t.TempDir(), t.TempDir()
	put(t, root, "products/alpha/spec.txt", "x")
	for name, tt := range map[string]struct{ file, data, roster string }{
		"duplicate":     {"r.txt", "alpha\nbeta\nalpha\n", `{"file":"r.txt"}`},
		"slash in name": {"r.txt", "products/alpha\n", `{"file":"r.txt"}`},
		"backslash":     {"r.txt", "a\\b\n", `{"file":"r.txt"}`},
		"escape item":   {"r.txt", "../x\n", `{"file":"r.txt","match":"item"}`},
		"unclean item":  {"r.txt", "products//alpha\n", `{"file":"r.txt","match":"item"}`},
		"not utf8":      {"r.txt", "alpha\n\xff\n", `{"file":"r.txt"}`},
		"no column":     {"r.csv", "id,sku\n1,alpha\n", `{"file":"r.csv","column":"name"}`},
		"empty cell":    {"r.csv", "sku\nalpha\n \n", `{"file":"r.csv","column":"sku"}`},
		"ragged csv":    {"r.csv", "sku,x\nalpha\n", `{"file":"r.csv","column":"sku"}`},
		"empty csv":     {"r.csv", "", `{"file":"r.csv","column":"sku"}`},
		"two columns":   {"r.csv", "sku,sku\na,b\n", `{"file":"r.csv","column":"sku"}`},
		"missing file":  {"other.txt", "alpha\n", `{"file":"r.txt"}`},
	} {
		dir := filepath.Join(cfg, strings.ReplaceAll(name, " ", "-"))
		put(t, dir, tt.file, tt.data)
		_, e := Scan(root, manifest(t, rosterManifest(tt.roster)), Options{MaxContentBytes: 1024, RosterDir: dir})
		if e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, e := Scan(root, manifest(t, rosterManifest(`{"file":"r.txt"}`)), Options{MaxContentBytes: 1024}); e == nil {
		t.Error("relative roster without a folder accepted")
	}
	for _, bad := range []string{`{"file":""}`, `{"file":"r.txt","match":"path"}`, `{"file":"r.txt","unlisted":"warn"}`, `{"file":"r.csv"}`, `{"file":"r.txt","column":"sku"}`, `{"file":"r.txt","extra":1}`} {
		if _, e := Decode(strings.NewReader(rosterManifest(bad))); e == nil {
			t.Errorf("accepted roster %s", bad)
		}
	}
}

func TestRosterLimits(t *testing.T) {
	var b strings.Builder
	for i := 0; i <= MaxRosterEntries; i++ {
		fmt.Fprintf(&b, "item%d\n", i)
	}
	if _, e := parseRoster([]byte(b.String()), Roster{File: "r.txt", Match: "name"}); e == nil || !strings.Contains(e.Error(), "entries") {
		t.Fatal("entry limit", e)
	}
	if _, e := parseRoster(make([]byte, MaxRosterBytes+1), Roster{File: "r.txt", Match: "name"}); e == nil {
		t.Fatal("size limit")
	}
	f := filepath.Join(t.TempDir(), "big.txt")
	if e := os.WriteFile(f, []byte(strings.Repeat("a\n", MaxRosterBytes/2+1)), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := loadRoster(Roster{File: f, Match: "name"}, ""); e == nil {
		t.Fatal("oversized file read")
	}
}

func TestEmptyRosterAndLongEntry(t *testing.T) {
	for name, tt := range map[string]struct{ file, data string }{
		"zero bytes":  {"r.txt", ""},
		"comments":    {"r.txt", "# nothing yet\n\n"},
		"bom only":    {"r.txt", "\xef\xbb\xbf"},
		"header only": {"r.csv", "slug\n"},
		"long entry":  {"r.txt", strings.Repeat("a", MaxRosterEntryBytes+1) + "\n"},
	} {
		r := Roster{File: tt.file, Match: "name"}
		if strings.HasSuffix(tt.file, ".csv") {
			r.Column = "slug"
		}
		if _, e := parseRoster([]byte(tt.data), r); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, e := parseRoster([]byte(strings.Repeat("a", MaxRosterEntryBytes)+"\n"), Roster{File: "r.txt", Match: "name"}); e != nil {
		t.Fatal(e)
	}
}
