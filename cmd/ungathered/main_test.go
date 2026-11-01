package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	root := t.TempDir()
	pack := filepath.Join(root, "pack")
	_ = os.Mkdir(pack, 0700)
	_ = os.WriteFile(filepath.Join(pack, "a"), []byte("a"), 0600)
	schema := filepath.Join(root, "manifest.json")
	_ = os.WriteFile(schema, []byte(`{"version":1,"collections":[{"name":"files","items":"*","kind":"file","require":[{"name":"companion","path":"{item}.meta"}]}]}`), 0600)
	for _, tt := range []struct {
		args     []string
		want     int
		contains string
	}{{[]string{"--version"}, 0, "UnGathered"}, {[]string{"--help"}, 0, ""}, {nil, 2, ""}, {[]string{"--manifest", schema, "--root", pack, "--format", "json"}, 1, `"incomplete"`}, {[]string{"--manifest", schema, "--root", pack, "--output", filepath.Join(pack, "report")}, 2, ""}, {[]string{"--manifest", schema, "--root", pack, "--format", "bogus"}, 2, ""}} {
		var out, err bytes.Buffer
		if got := run(tt.args, &out, &err); got != tt.want || !strings.Contains(out.String(), tt.contains) {
			t.Fatalf("%v: %d %s %s", tt.args, got, out.String(), err.String())
		}
	}
	report := filepath.Join(root, "report.html")
	var out, err bytes.Buffer
	if got := run([]string{"--manifest", schema, "--root", pack, "--format", "html", "--output", report}, &out, &err); got != 1 {
		t.Fatal(got, err.String())
	}
	if got := run([]string{"--manifest", schema, "--root", pack, "--output", report}, &out, &err); got != 2 {
		t.Fatal("overwrote report")
	}
	_ = os.WriteFile(filepath.Join(pack, "a.meta"), []byte("metadata"), 0600)
	// A narrower anchor avoids treating the companion itself as a new item.
	b, _ := os.ReadFile(schema)
	b = bytes.ReplaceAll(b, []byte(`"items":"*"`), []byte(`"items":"a"`))
	_ = os.WriteFile(schema, b, 0600)
	if got := run([]string{"--manifest", schema, "--root", pack}, &out, &err); got != 0 {
		t.Fatal(got, err.String())
	}
}

func TestReportFilesSummaryAndRoster(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "pack")
	cfg := filepath.Join(dir, "cfg")
	out := filepath.Join(dir, "out")
	for _, d := range []string{filepath.Join(pack, "products", "alpha"), cfg, out} {
		if e := os.MkdirAll(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	_ = os.WriteFile(filepath.Join(pack, "products", "alpha", "spec.txt"), []byte("x"), 0600)
	_ = os.WriteFile(filepath.Join(cfg, "skus.txt"), []byte("alpha\nbeta\n"), 0600)
	schema := filepath.Join(cfg, "manifest.json")
	_ = os.WriteFile(schema, []byte(`{"version":1,"collections":[{"name":"products","items":"products/*","kind":"directory","roster":{"file":"skus.txt"},"require":[{"name":"spec","path":"{item}/spec.txt"}]}]}`), 0600)
	summary := filepath.Join(out, "summary.md")
	_ = os.WriteFile(summary, []byte("earlier step\n"), 0600)
	var o, e bytes.Buffer
	// The roster is found next to the manifest, not in the working directory.
	got := run([]string{"--manifest", schema, "--root", pack, "--format", "github", "--json", filepath.Join(out, "r.json"), "--html", filepath.Join(out, "r.html"), "--summary", summary}, &o, &e)
	if got != 1 || !strings.Contains(o.String(), "::error title=UnGathered missing_item::products: beta") {
		t.Fatal(got, o.String(), e.String())
	}
	j, _ := os.ReadFile(filepath.Join(out, "r.json"))
	h, _ := os.ReadFile(filepath.Join(out, "r.html"))
	s, _ := os.ReadFile(summary)
	if !bytes.Contains(j, []byte(`"missing_item"`)) || !bytes.Contains(h, []byte("<html")) || !bytes.HasPrefix(s, []byte("earlier step\n### UnGathered")) {
		t.Fatalf("%s\n%s", j, s)
	}
	for name, args := range map[string][]string{
		"json inside root":    {"--json", filepath.Join(pack, "r.json")},
		"summary inside root": {"--summary", filepath.Join(pack, "products", "alpha", "spec.txt")},
		"summary must exist":  {"--summary", filepath.Join(out, "absent.md")},
		"same file twice":     {"--json", filepath.Join(out, "x.json"), "--html", filepath.Join(out, "x.json")},
		"existing json":       {"--json", filepath.Join(out, "r.json")},
		"bad format":          {"--format", "yaml"},
		"missing roster":      {"--manifest", filepath.Join(dir, "elsewhere.json")},
	} {
		if name == "missing roster" {
			b, _ := os.ReadFile(schema)
			_ = os.WriteFile(filepath.Join(dir, "elsewhere.json"), b, 0600)
		}
		full := append([]string{"--manifest", schema, "--root", pack, "--html", filepath.Join(out, "partial-"+strings.ReplaceAll(name, " ", "-")+".html")}, args...)
		if got := run(full, &o, &e); got != 2 {
			t.Errorf("%s: exit %d", name, got)
		}
		if _, err := os.Stat(filepath.Join(out, "partial-"+strings.ReplaceAll(name, " ", "-")+".html")); err == nil {
			t.Errorf("%s: left a partial report", name)
		}
	}
}

func TestReportFoldersMadeAfterCheck(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "pack")
	_ = os.MkdirAll(filepath.Join(pack, "a"), 0700)
	_ = os.WriteFile(filepath.Join(pack, "a", "x"), []byte("x"), 0600)
	schema := filepath.Join(dir, "m.json")
	_ = os.WriteFile(schema, []byte(`{"version":1,"collections":[{"name":"d","items":"*","kind":"directory","require":[{"name":"x","path":"{item}/x"}]}]}`), 0600)
	var o, e bytes.Buffer
	// Missing folders outside the root are made.
	if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(dir, "out", "deep", "r.json")}, &o, &e); got != 0 {
		t.Fatal(got, e.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "deep", "r.json")); err != nil {
		t.Fatal(err)
	}
	// A missing folder inside the root is refused and never made.
	if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(pack, "report", "r.json")}, &o, &e); got != 2 {
		t.Fatal("accepted a report inside the root")
	}
	if _, err := os.Stat(filepath.Join(pack, "report")); err == nil {
		t.Fatal("made a folder inside the root")
	}
	// A failed later target takes away the folders an earlier one made.
	if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(dir, "new1", "r.json"), "--summary", filepath.Join(dir, "absent.md")}, &o, &e); got != 2 {
		t.Fatal(got)
	}
	_ = os.WriteFile(filepath.Join(dir, "taken.html"), nil, 0600)
	if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(dir, "new2", "a", "r.json"), "--html", filepath.Join(dir, "taken.html")}, &o, &e); got != 2 {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(dir, "new2")); err == nil {
		t.Fatal("left folders after a failed write")
	}
	// A symlinked folder pointing into the root is still refused.
	if err := os.Symlink(pack, filepath.Join(dir, "link")); err == nil {
		if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(dir, "link", "sub", "r.json")}, &o, &e); got != 2 {
			t.Fatal("followed a symlink into the root")
		}
		if _, err := os.Stat(filepath.Join(pack, "sub")); err == nil {
			t.Fatal("made a folder inside the root through a symlink")
		}
	}
}

func TestReportRefusedThroughOtherCapitalisation(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "pack")
	_ = os.MkdirAll(filepath.Join(pack, "a"), 0700)
	_ = os.WriteFile(filepath.Join(pack, "a", "x"), []byte("x"), 0600)
	if _, err := os.Stat(filepath.Join(dir, "PACK")); err != nil {
		t.Skip("case-sensitive file system")
	}
	schema := filepath.Join(dir, "m.json")
	_ = os.WriteFile(schema, []byte(`{"version":1,"collections":[{"name":"d","items":"*","kind":"directory","require":[{"name":"x","path":"{item}/x"}]}]}`), 0600)
	var o, e bytes.Buffer
	if got := run([]string{"--manifest", schema, "--root", pack, "--json", filepath.Join(dir, "PACK", "r.json")}, &o, &e); got != 2 {
		t.Fatal("accepted a report inside the root under another capitalisation")
	}
	if _, err := os.Stat(filepath.Join(pack, "r.json")); err == nil {
		t.Fatal("wrote inside the root")
	}
}
