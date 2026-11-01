package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/softdevstudio/ungathered"
	"github.com/softdevstudio/ungathered/internal/scan"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ungathered", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifest := fs.String("manifest", "", "JSON manifest path (required); relative roster files are read from its folder")
	root := fs.String("root", ".", "collection root directory")
	format := fs.String("format", "text", "report format: "+strings.Join(scan.Formats, ", "))
	output := fs.String("output", "", "new report file outside root; default stdout")
	jsonOut := fs.String("json", "", "also write a JSON report to this new file outside root")
	htmlOut := fs.String("html", "", "also write an HTML report to this new file outside root")
	summary := fs.String("summary", "", "append a Markdown summary to this existing file outside root (for $GITHUB_STEP_SUMMARY)")
	max := fs.Int64("max-content-bytes", 1<<20, "maximum bytes per regex content check (1 to 1073741824)")
	version := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: ungathered --manifest collection.json --root pack [--format text|json|html|github|markdown] [--output NEW_FILE] [--json NEW_FILE] [--html NEW_FILE] [--summary EXISTING_FILE]\nExit: 0 complete, 1 declared gaps, 2 invalid input or operational errors.")
		fs.PrintDefaults()
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "ungathered:", err); return 2 }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *version {
		fmt.Fprintln(stdout, "UnGathered", ungathered.Version)
		return 0
	}
	if *manifest == "" || len(fs.Args()) != 0 {
		fs.Usage()
		return 2
	}
	known := false
	for _, f := range scan.Formats {
		known = known || f == *format
	}
	if !known {
		return fail(fmt.Errorf("format must be one of %s", strings.Join(scan.Formats, ", ")))
	}
	manifestPath, err := filepath.Abs(*manifest)
	if err != nil {
		return fail(err)
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return fail(err)
	}
	m, err := scan.Decode(file)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fail(err)
	}
	report, err := scan.Scan(*root, m, scan.Options{MaxContentBytes: *max, RosterDir: filepath.Dir(manifestPath)})
	if err != nil {
		return fail(err)
	}
	// Every file target is resolved and checked before anything is written.
	type target struct {
		path, format string
		appendTo     bool
	}
	var targets []target
	seen := map[string]bool{}
	for _, t := range []struct {
		flag, path, format string
		appendTo           bool
	}{{"output", *output, *format, false}, {"json", *jsonOut, "json", false}, {"html", *htmlOut, "html", false}, {"summary", *summary, "markdown", true}} {
		if t.path == "" {
			continue
		}
		dest, e := outsideRoot(t.path, report.Root, t.appendTo)
		if e != nil {
			return fail(fmt.Errorf("--%s: %w", t.flag, e))
		}
		if seen[dest] {
			return fail(fmt.Errorf("--%s names a file another option already writes", t.flag))
		}
		seen[dest] = true
		targets = append(targets, target{dest, t.format, t.appendTo})
	}
	// created holds files and folders this run made, newest last, so a
	// failure can take them away again.
	var created []string
	undo := func(err error) int {
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
		return fail(err)
	}
	for _, t := range targets {
		if !t.appendTo {
			// Missing report folders are made only now, after the scan and
			// after the target was checked to be outside the root.
			if e := mkdirs(filepath.Dir(t.path), &created); e != nil {
				return undo(e)
			}
		}
		flags := os.O_CREATE | os.O_EXCL | os.O_WRONLY
		if t.appendTo {
			flags = os.O_APPEND | os.O_WRONLY
		}
		f, e := os.OpenFile(t.path, flags, 0600)
		if e != nil {
			return undo(e)
		}
		if !t.appendTo {
			created = append(created, t.path)
		}
		e = scan.Render(f, report, t.format)
		if e == nil {
			e = f.Sync()
		}
		closeErr = f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return undo(e)
		}
	}
	if *output == "" {
		if err = scan.Render(stdout, report, *format); err != nil {
			return undo(err)
		}
	}
	if report.Status == "error" {
		return 2
	}
	if report.Status == "incomplete" {
		return 1
	}
	return 0
}

// mkdirs creates dir and any missing parents, recording each folder it made.
func mkdirs(dir string, created *[]string) error {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, d)
		if filepath.Dir(d) == d {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0755); err != nil {
			return err
		}
		*created = append(*created, missing[i])
	}
	return nil
}

// outsideRoot resolves a report target and refuses one inside the scanned
// root, including through a symlinked folder. For a new file, the nearest
// existing folder above it is resolved and the missing ones are appended,
// since they can't be symlinks yet. An appended file must exist itself.
func outsideRoot(p, root string, existing bool) (string, error) {
	dest, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if existing {
		if dest, err = filepath.EvalSymlinks(dest); err != nil {
			return "", err
		}
		info, err := os.Stat(dest)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a regular file", p)
		}
	} else {
		var rest []string
		dir := filepath.Dir(dest)
		for {
			if _, err := os.Lstat(dir); err == nil {
				break
			} else if !os.IsNotExist(err) {
				return "", err
			}
			if filepath.Dir(dir) == dir {
				return "", fmt.Errorf("no existing folder above %s", p)
			}
			rest = append([]string{filepath.Base(dir)}, rest...)
			dir = filepath.Dir(dir)
		}
		parent, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(parent)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not a folder", dir)
		}
		dest = filepath.Join(append(append([]string{parent}, rest...), filepath.Base(dest))...)
	}
	inside := errors.New("report files must be outside the collection root")
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		if !strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(dest)) {
			return dest, nil
		}
		return "", err
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", inside
	}
	// Names compare byte for byte above, but macOS and Windows folders are
	// usually case-insensitive: /Users/x/Pack is /Users/x/pack. Compare the
	// existing folders above the target with the root as files too.
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	for d := dest; ; d = filepath.Dir(d) {
		if info, err := os.Stat(d); err == nil && os.SameFile(info, rootInfo) {
			return "", inside
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return dest, nil
}
