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

type Options struct{ MaxContentBytes int64 }
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
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Items   []Item `json:"items"`
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
	for _, c := range m.Collections {
		group := Group{Name: c.Name, Pattern: c.Items, Items: []Item{}}
		for _, n := range nodes {
			if n.mode&os.ModeSymlink != 0 && anchorCanReach(c.Items, n.name) {
				report.Issues = append(report.Issues, Issue{Collection: c.Name, Path: n.name, Status: "unsafe_symlink", Detail: "anchor pattern reaches a symlink; it was not followed"})
				continue
			}
			match, _ := path.Match(c.Items, n.name)
			if !match {
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
		if i.Status == "below_minimum" {
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

func anchorCanReach(pattern, name string) bool {
	p, n := strings.Split(pattern, "/"), strings.Split(name, "/")
	if len(n) > len(p) {
		return false
	}
	for i, part := range n {
		ok, _ := path.Match(p[i], part)
		if !ok {
			return false
		}
	}
	return true
}
