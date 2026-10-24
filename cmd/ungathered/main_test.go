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
