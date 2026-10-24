package main

import (
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
	manifest := fs.String("manifest", "", "JSON manifest path (required)")
	root := fs.String("root", ".", "collection root directory")
	format := fs.String("format", "text", "text, json or html")
	output := fs.String("output", "", "new report file outside root; default stdout")
	max := fs.Int64("max-content-bytes", 1<<20, "maximum bytes per regex content check (1 to 1073741824)")
	version := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: ungathered --manifest collection.json --root pack [--format json|html|text] [--output NEW_FILE]\nExit: 0 complete, 1 declared gaps, 2 invalid input or operational errors.")
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
	if *format != "text" && *format != "json" && *format != "html" {
		return fail(fmt.Errorf("format must be text, json or html"))
	}
	file, err := os.Open(*manifest)
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
	report, err := scan.Scan(*root, m, scan.Options{MaxContentBytes: *max})
	if err != nil {
		return fail(err)
	}
	if *output == "" {
		if err = scan.Render(stdout, report, *format); err != nil {
			return fail(err)
		}
	} else {
		dest, e := filepath.Abs(*output)
		if e != nil {
			return fail(e)
		}
		parent, e := filepath.EvalSymlinks(filepath.Dir(dest))
		if e != nil {
			return fail(e)
		}
		dest = filepath.Join(parent, filepath.Base(dest))
		rel, e := filepath.Rel(report.Root, dest)
		if e != nil {
			if !strings.EqualFold(filepath.VolumeName(report.Root), filepath.VolumeName(dest)) {
				rel = ".."
			} else {
				return fail(e)
			}
		}
		if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fail(fmt.Errorf("report output must be outside the collection root"))
		}
		f, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return fail(e)
		}
		e = scan.Render(f, report, *format)
		if e == nil {
			e = f.Sync()
		}
		closeErr = f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			_ = os.Remove(dest)
			return fail(e)
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
