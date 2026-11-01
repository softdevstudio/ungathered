package scan

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxRosterBytes   = 16 << 20
	MaxRosterEntries = 100000
)

type rosterList struct {
	sha256  string
	entries []string
}

// RosterResult is the per-collection roster summary in a report.
type RosterResult struct {
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
	Match    string `json:"match"`
	Entries  int    `json:"entries"`
	Found    int    `json:"found"`
	Missing  int    `json:"missing"`
	Unlisted int    `json:"unlisted"`
}

func rosterPath(r Roster, base string) (string, error) {
	f := filepath.FromSlash(r.File)
	if filepath.IsAbs(f) {
		return f, nil
	}
	if base == "" {
		return "", fmt.Errorf("roster %s is relative, but no manifest folder was given", r.File)
	}
	return filepath.Join(base, f), nil
}

func loadRoster(r Roster, base string) (rosterList, error) {
	p, err := rosterPath(r, base)
	if err != nil {
		return rosterList{}, err
	}
	f, err := os.Open(p)
	if err != nil {
		return rosterList{}, fmt.Errorf("roster %s: %w", r.File, err)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxRosterBytes+1))
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return rosterList{}, fmt.Errorf("roster %s: %w", r.File, err)
	}
	list, err := parseRoster(data, r)
	if err != nil {
		return rosterList{}, fmt.Errorf("roster %s: %w", r.File, err)
	}
	return list, nil
}

func parseRoster(data []byte, r Roster) (rosterList, error) {
	if len(data) > MaxRosterBytes {
		return rosterList{}, fmt.Errorf("exceeds %d bytes", MaxRosterBytes)
	}
	h := sha256.Sum256(data)
	list := rosterList{sha256: hex.EncodeToString(h[:])}
	if !utf8.Valid(data) {
		return list, fmt.Errorf("is not UTF-8 text")
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	seen := map[string]int{}
	add := func(line int, v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("line %d: empty entry", line)
		}
		if err := rosterEntry(v, r.Match); err != nil {
			return fmt.Errorf("line %d: %q %w", line, v, err)
		}
		if first, ok := seen[v]; ok {
			return fmt.Errorf("line %d: %q repeats line %d", line, v, first)
		}
		if len(list.entries) == MaxRosterEntries {
			return fmt.Errorf("has more than %d entries", MaxRosterEntries)
		}
		seen[v] = line
		list.entries = append(list.entries, v)
		return nil
	}
	if strings.EqualFold(path.Ext(strings.ReplaceAll(r.File, "\\", "/")), ".csv") {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.ReuseRecord = true
		header, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return list, fmt.Errorf("has no header row")
		}
		if err != nil {
			return list, err
		}
		col := -1
		for i, h := range header {
			if strings.TrimSpace(h) == r.Column {
				if col >= 0 {
					return list, fmt.Errorf("has more than one %q column", r.Column)
				}
				col = i
			}
		}
		if col < 0 {
			return list, fmt.Errorf("has no %q column in its header row", r.Column)
		}
		for {
			rec, err := cr.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return list, err
			}
			line, _ := cr.FieldPos(col)
			if err = add(line, rec[col]); err != nil {
				return list, err
			}
		}
	} else {
		for i, line := range strings.Split(string(data), "\n") {
			v := strings.TrimSpace(line)
			if v == "" || strings.HasPrefix(v, "#") {
				continue
			}
			if err := add(i+1, v); err != nil {
				return list, err
			}
		}
	}
	sort.Strings(list.entries)
	return list, nil
}

// rosterEntry checks one entry against the key it is matched on.
func rosterEntry(v, match string) error {
	if strings.ContainsAny(v, "\x00\r\n") {
		return fmt.Errorf("has a control character")
	}
	if match == "item" {
		if path.Clean(v) != v || v == "." {
			return fmt.Errorf("is not a canonical relative path")
		}
		return relative(v)
	}
	if strings.ContainsAny(v, "/\\") {
		return fmt.Errorf("has a path separator; use match item for paths")
	}
	return nil
}

func rosterKey(item, match string) string {
	switch match {
	case "item":
		return item
	case "stem":
		base := path.Base(item)
		return strings.TrimSuffix(base, path.Ext(base))
	default:
		return path.Base(item)
	}
}
