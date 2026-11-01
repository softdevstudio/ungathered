package scan

import (
	"fmt"
	"path"
	"strings"
)

// Globs are canonical forward-slash patterns matched one path segment at a
// time. A segment is either a path.Match pattern or the whole-segment wildcard
// "**". As in .gitignore, "**" matches zero or more segments, except at the
// end of a pattern, where it matches one or more ("drafts/**" is everything
// inside drafts, not drafts itself).

const doubleStar = "**"

func validGlob(p string) error {
	if path.Clean(p) != p || p == "." {
		return fmt.Errorf("need a canonical relative glob without ./, repeated separators or trailing slash")
	}
	if err := relative(p); err != nil {
		return err
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if s == doubleStar {
			if i > 0 && segs[i-1] == doubleStar {
				return fmt.Errorf("repeated ** segments")
			}
			continue
		}
		if strings.Contains(s, doubleStar) {
			return fmt.Errorf("** must be a whole path segment, as in assets/**/*.png")
		}
		if _, err := path.Match(s, "probe"); err != nil {
			return err
		}
	}
	return nil
}

// globState runs the segment matcher over name and returns, for every pattern
// position i, whether the whole name can be consumed with the pattern having
// reached position i. reach[len(p)] means a full match.
func globState(p, n []string) []bool {
	// cur[i]: pattern prefix p[:i] matches the name segments consumed so far.
	cur := make([]bool, len(p)+1)
	cur[0] = true
	closeStars(p, cur)
	for _, seg := range n {
		next := make([]bool, len(p)+1)
		for i := 0; i < len(p); i++ {
			if !cur[i] {
				continue
			}
			if p[i] == doubleStar {
				// The star absorbs this segment and may absorb more.
				next[i] = true
				next[i+1] = true
				continue
			}
			if ok, _ := path.Match(p[i], seg); ok {
				next[i+1] = true
			}
		}
		closeStars(p, next)
		cur = next
	}
	return cur
}

// closeStars lets an interior "**" match zero segments. A trailing "**" must
// match at least one, so it is never skipped.
func closeStars(p []string, state []bool) {
	for i := 0; i < len(p)-1; i++ {
		if state[i] && p[i] == doubleStar {
			state[i+1] = true
		}
	}
}

func matchGlob(pattern, name string) bool {
	p := strings.Split(pattern, "/")
	return globState(p, strings.Split(name, "/"))[len(p)]
}

// globCanDescend reports whether some path strictly below name can match
// pattern.
func globCanDescend(pattern, name string) bool {
	p := strings.Split(pattern, "/")
	state := globState(p, strings.Split(name, "/"))
	for i := 0; i < len(p); i++ {
		if state[i] {
			return true
		}
	}
	return false
}

// globCoversDescendants reports whether every path strictly below name
// matches pattern. Only a pattern ending in "**" can do that.
func globCoversDescendants(pattern, name string) bool {
	p := strings.Split(pattern, "/")
	if p[len(p)-1] != doubleStar {
		return false
	}
	// Some ancestor-or-self of name must match the part before the final
	// "**", which then takes everything below it.
	head, n := p[:len(p)-1], strings.Split(name, "/")
	if len(head) == 0 {
		return true
	}
	for k := 1; k <= len(n); k++ {
		if globState(head, n[:k])[len(head)] {
			return true
		}
	}
	return false
}
