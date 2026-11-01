package scan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

const MaxManifestBytes = 4 << 20

type Manifest struct {
	Version     int          `json:"version"`
	Collections []Collection `json:"collections"`
	hash        string
}
type Collection struct {
	Name     string        `json:"name"`
	Items    string        `json:"items"`
	Kind     string        `json:"kind"`
	Exclude  []string      `json:"exclude,omitempty"`
	MinItems *int          `json:"min_items,omitempty"`
	Roster   *Roster       `json:"roster,omitempty"`
	Require  []Requirement `json:"require"`
}

// Roster names a list of items that must exist, kept outside the scanned
// collection (a product export, an issue plan). File is relative to the
// manifest's folder unless absolute.
type Roster struct {
	File     string `json:"file"`
	Match    string `json:"match,omitempty"`
	Column   string `json:"column,omitempty"`
	Unlisted string `json:"unlisted,omitempty"`
}
type Requirement struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind,omitempty"`
	MinBytes   int64  `json:"min_bytes,omitempty"`
	Contains   string `json:"contains,omitempty"`
	expression *regexp.Regexp
}

func Decode(r io.Reader) (Manifest, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxManifestBytes+1))
	if err != nil {
		return Manifest{}, err
	}
	if len(b) > MaxManifestBytes {
		return Manifest{}, fmt.Errorf("manifest exceeds %d bytes", MaxManifestBytes)
	}
	if err := uniqueKeys(b); err != nil {
		return Manifest{}, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var m Manifest
	if err = d.Decode(&m); err != nil {
		return m, fmt.Errorf("decode manifest: %w", err)
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return m, fmt.Errorf("manifest must contain exactly one JSON value")
	}
	h := sha256.Sum256(b)
	m.hash = hex.EncodeToString(h[:])
	if err = m.validate(); err != nil {
		return m, err
	}
	return m, nil
}
func (m *Manifest) validate() error {
	if m.Version != 1 {
		return fmt.Errorf("manifest version must be 1")
	}
	if len(m.Collections) == 0 || len(m.Collections) > 100 {
		return fmt.Errorf("need 1 to 100 collections")
	}
	names := map[string]bool{}
	for ci := range m.Collections {
		c := &m.Collections[ci]
		if strings.TrimSpace(c.Name) == "" || names[c.Name] {
			return fmt.Errorf("collection names must be nonempty and unique: %q", c.Name)
		}
		names[c.Name] = true
		if c.Kind != "file" && c.Kind != "directory" {
			return fmt.Errorf("collection %s kind must be file or directory", c.Name)
		}
		if err := validGlob(c.Items); err != nil {
			return fmt.Errorf("collection %s items: %w", c.Name, err)
		}
		if len(c.Exclude) > 100 {
			return fmt.Errorf("collection %s has more than 100 exclude patterns", c.Name)
		}
		for _, x := range c.Exclude {
			if err := validGlob(x); err != nil {
				return fmt.Errorf("collection %s exclude %q: %w", c.Name, x, err)
			}
		}
		if c.Roster != nil {
			if err := c.Roster.validate(); err != nil {
				return fmt.Errorf("collection %s roster: %w", c.Name, err)
			}
		}
		if c.MinItems != nil && *c.MinItems < 0 {
			return fmt.Errorf("min_items must be nonnegative")
		}
		if len(c.Require) == 0 || len(c.Require) > 100 {
			return fmt.Errorf("collection %s needs 1 to 100 requirements", c.Name)
		}
		rules := map[string]bool{}
		for ri := range c.Require {
			r := &c.Require[ri]
			if strings.TrimSpace(r.Name) == "" || rules[r.Name] {
				return fmt.Errorf("requirement names must be nonempty and unique within %s", c.Name)
			}
			rules[r.Name] = true
			if r.Kind == "" {
				r.Kind = "file"
			}
			if r.Kind != "file" && r.Kind != "directory" {
				return fmt.Errorf("requirement %s has invalid kind", r.Name)
			}
			if r.MinBytes < 0 {
				return fmt.Errorf("min_bytes must be nonnegative")
			}
			if r.Kind == "directory" && (r.MinBytes != 0 || r.Contains != "") {
				return fmt.Errorf("directory requirements cannot have min_bytes or contains")
			}
			expanded := strings.NewReplacer("{item}", "item", "{name}", "name", "{stem}", "stem", "{dir}", "dir").Replace(r.Path)
			if strings.ContainsAny(expanded, "{}") {
				return fmt.Errorf("requirement %s has an unknown path variable", r.Name)
			}
			if err := relative(expanded); err != nil {
				return fmt.Errorf("requirement %s path: %w", r.Name, err)
			}
			if len(r.Contains) > 4096 {
				return fmt.Errorf("contains pattern exceeds 4096 bytes")
			}
			r.expression = nil
			if r.Contains != "" {
				var err error
				r.expression, err = regexp.Compile(r.Contains)
				if err != nil {
					return fmt.Errorf("requirement %s contains: %w", r.Name, err)
				}
			}
		}
	}
	return nil
}
func (r *Roster) validate() error {
	if strings.TrimSpace(r.File) == "" || strings.ContainsRune(r.File, 0) {
		return fmt.Errorf("file is required")
	}
	if r.Match == "" {
		r.Match = "name"
	}
	if r.Match != "name" && r.Match != "stem" && r.Match != "item" {
		return fmt.Errorf("match must be name, stem or item")
	}
	if r.Unlisted == "" {
		r.Unlisted = "ignore"
	}
	if r.Unlisted != "ignore" && r.Unlisted != "gap" {
		return fmt.Errorf("unlisted must be ignore or gap")
	}
	csv := strings.EqualFold(path.Ext(strings.ReplaceAll(r.File, "\\", "/")), ".csv")
	if csv && strings.TrimSpace(r.Column) == "" {
		return fmt.Errorf("a .csv roster needs column, the header of the column holding item names")
	}
	if !csv && r.Column != "" {
		return fmt.Errorf("column applies only to .csv rosters")
	}
	if len(r.Column) > 256 {
		return fmt.Errorf("column exceeds 256 bytes")
	}
	return nil
}
func relative(s string) error {
	if s == "" || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\:\x00") {
		return fmt.Errorf("need a relative forward-slash path without drive names")
	}
	for _, part := range strings.Split(s, "/") {
		if part == ".." {
			return fmt.Errorf("parent traversal is forbidden")
		}
	}
	return nil
}

// Reject duplicate object keys before decoding the schema so a later key cannot
// silently replace a collection or requirement. Depth is bounded independently.
func uniqueKeys(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("manifest nesting exceeds 64 levels")
		}
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				for _, ch := range name {
					if (ch < 'a' || ch > 'z') && ch != '_' {
						return fmt.Errorf("schema keys must use lowercase ASCII: %q", name)
					}
				}
				canonical := strings.ToLower(name)
				if seen[canonical] {
					return fmt.Errorf("duplicate JSON key %q", name)
				}
				seen[canonical] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	return value(0)
}
