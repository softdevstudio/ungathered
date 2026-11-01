package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	for _, tt := range []struct {
		pattern, name string
		want          bool
	}{
		{"assets/**/*.png", "assets/a.png", true},
		{"assets/**/*.png", "assets/x/y/z/a.png", true},
		{"assets/**/*.png", "assets/x/a.jpg", false},
		{"assets/**/*.png", "other/a.png", false},
		{"**/*.md", "a.md", true},
		{"**/*.md", "x/y/a.md", true},
		{"**/spec", "spec", true},
		{"drafts/**", "drafts", false},
		{"drafts/**", "drafts/a", true},
		{"drafts/**", "drafts/a/b/c", true},
		{"**", "a", true},
		{"**", "a/b", true},
		{"a/**/b/**/c", "a/b/c", true},
		{"a/**/b/**/c", "a/x/b/y/z/c", true},
		{"a/**/b/**/c", "a/x/c", false},
		{"products/*", "products/a/b", false},
		{"products/*", "products/a", true},
		{"p?/[ab]", "px/a", true},
	} {
		if got := matchGlob(tt.pattern, tt.name); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestGlobDescend(t *testing.T) {
	for _, tt := range []struct {
		pattern, name       string
		descend, coversDesc bool
	}{
		{"assets/**/*.png", "assets", true, false},
		{"assets/**/*.png", "assets/x/y", true, false},
		{"assets/**/*.png", "other", false, false},
		{"products/*", "products", true, false},
		{"products/*", "products/a", false, false},
		{"drafts/**", "drafts", true, true},
		{"drafts/**", "drafts/a", true, true},
		{"drafts/**", "other", false, false},
		{"**/old/**", "x/old", true, true},
		{"**/old/**", "x/new", true, false},
	} {
		if got := globCanDescend(tt.pattern, tt.name); got != tt.descend {
			t.Errorf("globCanDescend(%q, %q) = %v", tt.pattern, tt.name, got)
		}
		if got := globCoversDescendants(tt.pattern, tt.name); got != tt.coversDesc {
			t.Errorf("globCoversDescendants(%q, %q) = %v", tt.pattern, tt.name, got)
		}
	}
}

func TestInvalidGlobs(t *testing.T) {
	for _, p := range []string{"a**b", "**.md", "assets/**/**/x", "a/**x", "../**", "/abs/**", "a//b", "a/", "./a", "[bad", "a\\b"} {
		if err := validGlob(p); err == nil {
			t.Errorf("accepted %q", p)
		}
		s := strings.Replace(docs, `"items":"products/*"`, `"items":"products/*","exclude":["`+strings.ReplaceAll(p, "\\", "\\\\")+`"]`, 1)
		if _, e := Decode(strings.NewReader(s)); e == nil {
			t.Errorf("accepted exclude %q", p)
		}
	}
}

const nested = `{"version":1,"collections":[{"name":"images","items":"assets/**/*.png","kind":"file","exclude":["assets/drafts/**","**/thumb-*.png"],"require":[{"name":"credit","path":"{dir}/{stem}.credit.txt","min_bytes":1}]}]}`

func TestRecursiveAnchorsAndExcludes(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/logo.png", "png")
	put(t, root, "assets/logo.credit.txt", "Studio")
	put(t, root, "assets/2026/spring/cover.png", "png")
	put(t, root, "assets/2026/spring/thumb-cover.png", "png")
	put(t, root, "assets/drafts/wip.png", "png")
	put(t, root, "assets/drafts/deep/wip.png", "png")
	put(t, root, "other/stray.png", "png")
	r, e := Scan(root, manifest(t, nested), Options{MaxContentBytes: 1024})
	if e != nil {
		t.Fatal(e)
	}
	var got []string
	for _, it := range r.Collections[0].Items {
		got = append(got, it.Path)
	}
	if strings.Join(got, ",") != "assets/2026/spring/cover.png,assets/logo.png" {
		t.Fatalf("items %v", got)
	}
	if r.Status != "incomplete" || r.Totals.Gaps != 1 || r.Totals.Present != 1 {
		t.Fatalf("%+v", r.Totals)
	}
	if c := r.Collections[0].Items[0].Checks[0]; c.Path != "assets/2026/spring/cover.credit.txt" || c.Status != "missing" {
		t.Fatalf("%+v", c)
	}
	if strings.Join(r.Collections[0].Exclude, ",") != "assets/drafts/**,**/thumb-*.png" || r.Collections[0].Kind != "file" {
		t.Fatalf("%+v", r.Collections[0])
	}
}

func TestDirectoryAnchorsAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	put(t, root, "catalog/a/spec.txt", "x")
	put(t, root, "catalog/line/b/spec.txt", "x")
	put(t, root, "catalog/line/b/old/spec.txt", "x")
	m := manifest(t, `{"version":1,"collections":[{"name":"parts","items":"catalog/**/*","kind":"directory","exclude":["**/old"],"require":[{"name":"spec","path":"{item}/spec.txt"}]}]}`)
	r, e := Scan(root, m, Options{MaxContentBytes: 1024})
	if e != nil {
		t.Fatal(e)
	}
	// catalog/line has no spec: a ** pattern selects every folder it reaches,
	// which is why excludes exist.
	if r.Totals.Items != 3 || r.Totals.Gaps != 1 || r.Collections[0].Items[1].Path != "catalog/line" {
		t.Fatalf("%+v", r.Collections[0].Items)
	}
}

func TestSymlinksWithRecursiveGlobs(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	put(t, outside, "hidden.png", "png")
	put(t, root, "assets/a.png", "png")
	put(t, root, "assets/a.credit.txt", "credit")
	if e := os.Symlink(outside, filepath.Join(root, "assets/linked")); e != nil {
		t.Skip(e)
	}
	// A symlinked folder under a ** pattern could hold anchors: error.
	r, e := Scan(root, manifest(t, nested), Options{MaxContentBytes: 1024})
	if e != nil || r.Status != "error" || r.Issues[0].Status != "unsafe_symlink" || r.Issues[0].Path != "assets/linked" {
		t.Fatalf("%+v %v", r, e)
	}
	// Under an excluded tree nothing below it can count, so it is skipped.
	if e := os.Remove(filepath.Join(root, "assets/linked")); e != nil {
		t.Fatal(e)
	}
	put(t, root, "assets/drafts/keep.txt", "x")
	if e := os.Symlink(outside, filepath.Join(root, "assets/drafts/linked")); e != nil {
		t.Fatal(e)
	}
	r, e = Scan(root, manifest(t, nested), Options{MaxContentBytes: 1024})
	if e != nil || r.Status != "complete" || len(r.Issues) != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	// A symlink outside every pattern's reach is ignored, as in 0.1.
	if e := os.Symlink(outside, filepath.Join(root, "unrelated")); e != nil {
		t.Fatal(e)
	}
	r, e = Scan(root, manifest(t, nested), Options{MaxContentBytes: 1024})
	if e != nil || r.Status != "complete" {
		t.Fatalf("%+v %v", r, e)
	}
	// An exclude that only drops some names below the link does not make it safe.
	m := manifest(t, strings.Replace(nested, `"assets/drafts/**",`, ``, 1))
	r, e = Scan(root, m, Options{MaxContentBytes: 1024})
	if e != nil || r.Status != "error" {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestExcludeEverythingIsBelowMinimum(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/drafts/a.png", "png")
	r, e := Scan(root, manifest(t, nested), Options{MaxContentBytes: 1024})
	if e != nil || r.Status != "incomplete" || r.Issues[0].Status != "below_minimum" {
		t.Fatalf("%+v %v", r, e)
	}
}
