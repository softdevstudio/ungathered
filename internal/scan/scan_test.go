package scan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func manifest(t *testing.T, s string) Manifest {
	t.Helper()
	m, e := Decode(strings.NewReader(s))
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func put(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
}

const docs = `{"version":1,"collections":[{"name":"packs","items":"products/*","kind":"directory","require":[{"name":"spec","path":"{item}/spec.txt","min_bytes":1},{"name":"drawing","path":"{item}/drawing.txt"}]}]}`

func TestManufacturingGapsAndCompletion(t *testing.T) {
	root := t.TempDir()
	put(t, root, "products/beta/spec.txt", "ok")
	put(t, root, "products/alpha/spec.txt", "")
	put(t, root, "products/alpha/drawing.txt", "drawing")
	m := manifest(t, docs)
	r, e := Scan(root, m, Options{1024})
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "incomplete" || r.Totals.Gaps != 2 || r.Totals.Checks != 4 || r.Collections[0].Items[0].Path != "products/alpha" {
		t.Fatalf("%+v", r)
	}
	put(t, root, "products/beta/drawing.txt", "drawing")
	put(t, root, "products/alpha/spec.txt", "ok")
	r, e = Scan(root, m, Options{1024})
	if e != nil || r.Status != "complete" || r.Totals.CompleteItems != 2 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestPublishingAndVariables(t *testing.T) {
	root := t.TempDir()
	put(t, root, "articles/cafe.md", "story\nPhoto credit: author\n")
	put(t, root, "images/cafe.jpg", "image")
	m := manifest(t, `{"version":1,"collections":[{"name":"posts","items":"articles/*.md","kind":"file","require":[{"name":"image","path":"images/{stem}.jpg","min_bytes":1},{"name":"credit","path":"{dir}/{name}","contains":"(?m)^Photo credit: .+"}]}]}`)
	r, e := Scan(root, m, Options{1024})
	if e != nil || r.Status != "complete" {
		t.Fatalf("%+v %v", r, e)
	}
	put(t, root, "articles/cafe.md", "story without credit")
	r, e = Scan(root, m, Options{1024})
	if e != nil || r.Totals.Gaps != 1 || r.Collections[0].Items[0].Checks[1].Status != "content_mismatch" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestEmptyCollectionIsNotCompleteByDefault(t *testing.T) {
	root := t.TempDir()
	r, e := Scan(root, manifest(t, docs), Options{1024})
	if e != nil || r.Status != "incomplete" || r.Totals.Gaps != 1 || r.Issues[0].Status != "below_minimum" {
		t.Fatalf("%+v %v", r, e)
	}
	m := manifest(t, strings.Replace(docs, `"kind":"directory"`, `"kind":"directory","min_items":0`, 1))
	r, e = Scan(root, m, Options{1024})
	if e != nil || r.Status != "complete" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestWrongTypeAndReadLimit(t *testing.T) {
	root := t.TempDir()
	put(t, root, "items/a", "text")
	put(t, root, "companions/a/file", "x")
	m := manifest(t, `{"version":1,"collections":[{"name":"files","items":"items/*","kind":"file","require":[{"name":"companion","path":"companions/{stem}"},{"name":"content","path":"{item}","contains":"text"}]}]}`)
	r, e := Scan(root, m, Options{2})
	if e != nil || r.Status != "error" || r.Totals.Gaps != 1 || r.Totals.Errors != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	if r.Collections[0].Items[0].Checks[0].Status != "wrong_type" || r.Collections[0].Items[0].Checks[1].Status != "content_too_large" {
		t.Fatal(r)
	}
}
func TestSymlinksNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	put(t, outside, "secret", "credit")
	if e := os.Symlink(outside, filepath.Join(root, "linked")); e != nil {
		t.Skip(e)
	}
	put(t, root, "items/a", "a")
	m := manifest(t, `{"version":1,"collections":[{"name":"files","items":"items/*","kind":"file","require":[{"name":"outside","path":"linked/secret"}]}]}`)
	r, e := Scan(root, m, Options{1024})
	if e != nil || r.Status != "error" || r.Collections[0].Items[0].Checks[0].Status != "unsafe_symlink" {
		t.Fatalf("%+v %v", r, e)
	}
	m.Collections[0].Items = "linked"
	m.Collections[0].Kind = "directory"
	r, e = Scan(root, m, Options{1024})
	if e != nil || r.Status != "error" || r.Totals.Items != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	if _, e = Scan(filepath.Join(root, "linked"), m, Options{1024}); e == nil {
		t.Fatal("accepted symlink root")
	}
}
func TestDirectoryRequirementAndMissingParent(t *testing.T) {
	root := t.TempDir()
	put(t, root, "items/a", "x")
	_ = os.Mkdir(filepath.Join(root, "folder"), 0700)
	m := manifest(t, `{"version":1,"collections":[{"name":"files","items":"items/*","kind":"file","require":[{"name":"folder","path":"folder","kind":"directory"},{"name":"missing","path":"absent/nested/file"}]}]}`)
	r, e := Scan(root, m, Options{1024})
	if e != nil || r.Totals.Present != 1 || r.Totals.Gaps != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestDeterminismAndNoMutation(t *testing.T) {
	root := t.TempDir()
	put(t, root, "products/z/spec.txt", "original bytes")
	before, _ := os.ReadFile(filepath.Join(root, "products/z/spec.txt"))
	m := manifest(t, docs)
	r1, e := Scan(root, m, Options{1024})
	if e != nil {
		t.Fatal(e)
	}
	r2, e := Scan(root, m, Options{1024})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(r1)
	b, _ := json.Marshal(r2)
	if !bytes.Equal(a, b) {
		t.Fatal("nondeterministic report")
	}
	after, _ := os.ReadFile(filepath.Join(root, "products/z/spec.txt"))
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
}
func TestManifestFailures(t *testing.T) {
	for _, s := range []string{`{}`, `{"version":2,"collections":[]}`, strings.Replace(docs, `"items":"products/*"`, `"items":"../*"`, 1), strings.Replace(docs, `"items":"products/*"`, `"items":"[bad"`, 1), strings.Replace(docs, `"items":"products/*"`, `"items":"products/**"`, 1), strings.Replace(docs, `"{item}/spec.txt"`, `"/absolute"`, 1), strings.Replace(docs, `"{item}/spec.txt"`, `"{unknown}"`, 1), strings.Replace(docs, `"min_bytes":1`, `"contains":"[bad"`, 1), strings.Replace(docs, `"min_bytes":1`, `"min_bytes":-1`, 1), strings.Replace(docs, `"version":1`, `"version":1,"unknown":1`, 1), docs + `{}`} {
		if _, e := Decode(strings.NewReader(s)); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestReportFormatsAndEscaping(t *testing.T) {
	r := Report{ToolVersion: "test", Root: "<script>alert(1)</script>", Status: "error", Issues: []Issue{{Path: "<img>", Status: "scan_error", Detail: "<script>"}}}
	for _, format := range []string{"text", "json", "html"} {
		var b bytes.Buffer
		if e := Render(&b, r, format); e != nil {
			t.Fatal(e)
		}
		if format == "html" && strings.Contains(b.String(), "<script>") {
			t.Fatal("HTML injection")
		}
	}
	if e := Render(&bytes.Buffer{}, r, "bad"); e == nil {
		t.Fatal("bad format accepted")
	}
}

func TestNoncanonicalAndDuplicateManifestsRejected(t *testing.T) {
	for _, pattern := range []string{"./products/*", "products///*", "products/*/", "."} {
		s := strings.Replace(docs, `"items":"products/*"`, `"items":"`+pattern+`","min_items":0`, 1)
		if _, e := Decode(strings.NewReader(s)); e == nil {
			t.Fatal("accepted noncanonical glob", pattern)
		}
	}
	for _, s := range []string{strings.Replace(docs, `"version":1`, `"VERSION":2,"version":1`, 1), strings.Replace(docs, `"items":"products/*"`, `"items":"products/*","itemſ":"none/*"`, 1), strings.Replace(docs, `"version":1`, `"version":2,"version":1`, 1), strings.Replace(docs, `"name":"spec"`, `"name":"hidden","name":"spec"`, 1)} {
		if _, e := Decode(strings.NewReader(s)); e == nil {
			t.Fatal("accepted duplicate keys")
		}
	}
}
func TestSymlinkCannotHideAnchorDescendants(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	put(t, root, "articles/real/a.md", "ok")
	put(t, outside, "hidden.md", "secret")
	if e := os.Symlink(outside, filepath.Join(root, "articles/linked")); e != nil {
		t.Skip(e)
	}
	m := manifest(t, `{"version":1,"collections":[{"name":"articles","items":"articles/*/*.md","kind":"file","require":[{"name":"exists","path":"{item}"}]}]}`)
	r, e := Scan(root, m, Options{1024})
	if e != nil || r.Status != "error" || r.Totals.Errors != 1 || r.Totals.CompleteItems != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestConcurrentScansShareReadOnlyManifest(t *testing.T) {
	root := t.TempDir()
	put(t, root, "products/a/spec.txt", "ok")
	put(t, root, "products/a/drawing.txt", "ok")
	m := manifest(t, docs)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := Scan(root, m, Options{1024})
			if e != nil {
				errs <- e
			} else if r.Status != "complete" {
				errs <- fmt.Errorf("unexpected status %s", r.Status)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
