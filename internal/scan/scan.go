package scan

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/softdevstudio/ungathered"
)

type Options struct {
	MaxContentBytes int64
	// RosterDir is the folder relative roster files are read from, normally
	// the manifest's folder.
	RosterDir string
}
type Check struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Bytes  int64  `json:"bytes"`
	Detail string `json:"detail,omitempty"`
}
type Item struct {
	Path     string  `json:"path"`
	Complete bool    `json:"complete"`
	Checks   []Check `json:"checks"`
}
type Group struct {
	Name    string        `json:"name"`
	Pattern string        `json:"pattern"`
	Kind    string        `json:"kind"`
	Exclude []string      `json:"exclude,omitempty"`
	Roster  *RosterResult `json:"roster,omitempty"`
	Items   []Item        `json:"items"`
}
type Issue struct {
	Collection string `json:"collection,omitempty"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}
type Totals struct {
	Items         int `json:"items"`
	CompleteItems int `json:"complete_items"`
	Checks        int `json:"checks"`
	Present       int `json:"present"`
	Gaps          int `json:"gaps"`
	Errors        int `json:"errors"`
}
type Report struct {
	Version        int     `json:"version"`
	ToolVersion    string  `json:"tool_version"`
	Root           string  `json:"root"`
	ManifestSHA256 string  `json:"manifest_sha256"`
	Status         string  `json:"status"`
	Totals         Totals  `json:"totals"`
	Collections    []Group `json:"collections"`
	Issues         []Issue `json:"issues"`
}
type node struct {
	name string
	mode fs.FileMode
}

func Scan(root string, m Manifest, opts Options) (Report, error) {
	m.Collections = append([]Collection(nil), m.Collections...)
	for i := range m.Collections {
		m.Collections[i].Require = append([]Requirement(nil), m.Collections[i].Require...)
		m.Collections[i].Exclude = append([]string(nil), m.Collections[i].Exclude...)
		if r := m.Collections[i].Roster; r != nil {
			copied := *r
			m.Collections[i].Roster = &copied
		}
	}
	if err := m.validate(); err != nil {
		return Report{}, err
	}
	if opts.MaxContentBytes <= 0 || opts.MaxContentBytes > 1<<30 {
		return Report{}, fmt.Errorf("max content bytes must be between 1 and 1073741824")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Report{}, fmt.Errorf("root must be a directory, not a symlink")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Report{}, err
	}
	rosters := make([]rosterList, len(m.Collections))
	for i, c := range m.Collections {
		if c.Roster != nil {
			if rosters[i], err = loadRoster(*c.Roster, opts.RosterDir); err != nil {
				return Report{}, err
			}
		}
	}
	report := Report{Version: 1, ToolVersion: ungathered.Version, Root: root, ManifestSHA256: m.hash, Status: "complete", Collections: []Group{}, Issues: []Issue{}}
	var nodes []node
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		rel, re := filepath.Rel(root, p)
		if re != nil {
			report.Issues = append(report.Issues, Issue{Path: p, Status: "scan_error", Detail: re.Error()})
			return nil
		}
		rel = filepath.ToSlash(rel)
		if e != nil {
			report.Issues = append(report.Issues, Issue{Path: rel, Status: "scan_error", Detail: e.Error()})
			return nil
		}
		if rel == "." {
			return nil
		}
		i, e := d.Info()
		if e != nil {
			report.Issues = append(report.Issues, Issue{Path: rel, Status: "scan_error", Detail: e.Error()})
			return nil
		}
		nodes = append(nodes, node{rel, i.Mode()})
		return nil
	})
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].name < nodes[j].name })
	for ci, c := range m.Collections {
		group := Group{Name: c.Name, Pattern: c.Items, Kind: c.Kind, Exclude: c.Exclude, Items: []Item{}}
		for _, n := range nodes {
			if n.mode&os.ModeSymlink != 0 {
				if symlinkCanHide(c, n.name) {
					report.Issues = append(report.Issues, Issue{Collection: c.Name, Path: n.name, Status: "unsafe_symlink", Detail: "anchor pattern reaches a symlink; it was not followed"})
				}
				continue
			}
			if !matchGlob(c.Items, n.name) || excluded(c, n.name) {
				continue
			}

			if c.Kind == "directory" && !n.mode.IsDir() || c.Kind == "file" && !n.mode.IsRegular() {
				continue
			}
			item := Item{Path: n.name, Complete: true, Checks: []Check{}}
			vars := strings.NewReplacer("{item}", n.name, "{name}", path.Base(n.name), "{stem}", strings.TrimSuffix(path.Base(n.name), path.Ext(n.name)), "{dir}", path.Dir(n.name))
			for _, r := range c.Require {
				target := vars.Replace(r.Path)
				check := Check{Name: r.Name, Path: path.Clean(target)}
				if e := relative(target); e != nil {
					check.Status = "unsafe_path"
					check.Detail = e.Error()
				} else {
					check = checkPath(root, check, r, opts)
				}
				report.Totals.Checks++
				if check.Status == "present" {
					report.Totals.Present++
				} else {
					item.Complete = false
					if operational(check.Status) {
						report.Totals.Errors++
					} else {
						report.Totals.Gaps++
					}
				}
				item.Checks = append(item.Checks, check)
			}
			report.Totals.Items++
			if item.Complete {
				report.Totals.CompleteItems++
			}
			group.Items = append(group.Items, item)
		}
		if c.Roster != nil {
			report.Issues = append(report.Issues, compareRoster(c, rosters[ci], &group)...)
		}
		min := 1
		if c.MinItems != nil {
			min = *c.MinItems
		}
		if len(group.Items) < min {
			report.Issues = append(report.Issues, Issue{Collection: c.Name, Path: c.Items, Status: "below_minimum", Detail: fmt.Sprintf("matched %d %s items; need at least %d", len(group.Items), c.Kind, min)})
		}
		report.Collections = append(report.Collections, group)
	}
	for _, i := range report.Issues {
		if declaredGap(i.Status) {
			report.Totals.Gaps++
		} else {
			report.Totals.Errors++
		}
	}
	if report.Totals.Errors > 0 {
		report.Status = "error"
	} else if report.Totals.Gaps > 0 {
		report.Status = "incomplete"
	}
	return report, nil
}
func declaredGap(s string) bool {
	return s == "below_minimum" || s == "missing_item" || s == "unlisted_item"
}
func excluded(c Collection, name string) bool {
	for _, x := range c.Exclude {
		if matchGlob(x, name) {
			return true
		}
	}
	return false
}

// symlinkCanHide reports whether a symlink that is never followed could be,
// or could stand in for a folder holding, an anchor this collection counts.
func symlinkCanHide(c Collection, name string) bool {
	if matchGlob(c.Items, name) && !excluded(c, name) {
		return true
	}
	if !globCanDescend(c.Items, name) {
		return false
	}
	for _, x := range c.Exclude {
		if globCoversDescendants(x, name) {
			return false
		}
	}
	return true
}

func compareRoster(c Collection, list rosterList, g *Group) []Issue {
	res := &RosterResult{File: c.Roster.File, SHA256: list.sha256, Match: c.Roster.Match, Entries: len(list.entries)}
	g.Roster = res
	listed := make(map[string]bool, len(list.entries))
	for _, e := range list.entries {
		listed[e] = true
	}
	found := map[string]bool{}
	var issues, unlisted []Issue
	for _, it := range g.Items {
		k := rosterKey(it.Path, c.Roster.Match)
		if listed[k] {
			found[k] = true
		} else if c.Roster.Unlisted == "gap" {
			unlisted = append(unlisted, Issue{Collection: c.Name, Path: it.Path, Status: "unlisted_item", Detail: fmt.Sprintf("selected item is not in roster %s", c.Roster.File)})
		}
	}
	for _, e := range list.entries {
		if !found[e] {
			issues = append(issues, Issue{Collection: c.Name, Path: e, Status: "missing_item", Detail: fmt.Sprintf("listed in roster %s; no selected item matches its %s", c.Roster.File, c.Roster.Match)})
		}
	}
	res.Found = len(found)
	res.Missing = len(issues)
	res.Unlisted = len(unlisted)
	return append(issues, unlisted...)
}
func operational(s string) bool {
	return s == "unreadable" || s == "unsafe_symlink" || s == "unsafe_path" || s == "content_too_large"
}
func checkPath(root string, c Check, r Requirement, opts Options) Check {
	current := root
	for _, part := range strings.Split(c.Path, "/") {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.Status = "missing"
				c.Detail = "required path does not exist"
			} else {
				c.Status = "unreadable"
				c.Detail = err.Error()
			}
			return c
		}
		if info.Mode()&os.ModeSymlink != 0 {
			c.Status = "unsafe_symlink"
			c.Detail = "required path contains a symlink; it was not followed"
			return c
		}
		if current != filepath.Join(root, filepath.FromSlash(c.Path)) && !info.IsDir() {
			c.Status = "wrong_type"
			c.Detail = "a parent path is not a directory"
			return c
		}
	}
	info, err := os.Lstat(current)
	if err != nil {
		c.Status = "unreadable"
		c.Detail = err.Error()
		return c
	}
	if r.Kind == "directory" {
		if info.IsDir() {
			c.Status = "present"
		} else {
			c.Status = "wrong_type"
			c.Detail = "expected directory"
		}
		return c
	}
	if !info.Mode().IsRegular() {
		c.Status = "wrong_type"
		c.Detail = "expected regular file"
		return c
	}
	c.Bytes = info.Size()
	if info.Size() < r.MinBytes {
		c.Status = "too_small"
		c.Detail = fmt.Sprintf("need at least %d bytes", r.MinBytes)
		return c
	}
	if r.expression != nil {
		if info.Size() > opts.MaxContentBytes {
			c.Status = "content_too_large"
			c.Detail = "content check exceeds configured read limit"
			return c
		}
		f, e := os.Open(current)
		if e != nil {
			c.Status = "unreadable"
			c.Detail = e.Error()
			return c
		}
		data, e := io.ReadAll(io.LimitReader(f, opts.MaxContentBytes+1))
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			c.Status = "unreadable"
			c.Detail = e.Error()
			return c
		}
		if int64(len(data)) > opts.MaxContentBytes {
			c.Status = "content_too_large"
			c.Detail = "file grew beyond configured read limit"
			return c
		}
		if !r.expression.Match(data) {
			c.Status = "content_mismatch"
			c.Detail = "required content pattern did not match"
			return c
		}
	}
	c.Status = "present"
	return c
}
