package scan

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Formats lists the report formats Render accepts.
var Formats = []string{"text", "json", "html", "github", "markdown"}

func Render(w io.Writer, r Report, format string) error {
	switch format {
	case "json":
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(r)
	case "text":
		if _, err := fmt.Fprintf(w, "UnGathered %s\nStatus: %s\nItems: %d, complete: %d, checks: %d, present: %d, gaps: %d, errors: %d\n", r.ToolVersion, r.Status, r.Totals.Items, r.Totals.CompleteItems, r.Totals.Checks, r.Totals.Present, r.Totals.Gaps, r.Totals.Errors); err != nil {
			return err
		}
		for _, g := range r.Collections {
			if g.Roster != nil {
				if _, err := fmt.Fprintf(w, "Roster %s for %s: %d entries, %d found, %d missing, %d unlisted\n", g.Roster.File, g.Name, g.Roster.Entries, g.Roster.Found, g.Roster.Missing, g.Roster.Unlisted); err != nil {
					return err
				}
			}
		}
		for _, i := range r.Issues {
			if _, err := fmt.Fprintf(w, "[%s] %s %s: %s\n", i.Status, i.Collection, i.Path, i.Detail); err != nil {
				return err
			}
		}
		for _, g := range r.Collections {
			for _, item := range g.Items {
				for _, c := range item.Checks {
					if c.Status != "present" {
						if _, err := fmt.Fprintf(w, "[%s] %s / %s / %s: %s (%s)\n", c.Status, g.Name, item.Path, c.Name, c.Path, c.Detail); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	case "html":
		return htmlReport.Execute(w, r)
	case "github":
		return renderGitHub(w, r, os.Getenv("GITHUB_WORKSPACE"))
	case "markdown":
		return renderMarkdown(w, r)
	default:
		return fmt.Errorf("format must be one of %s", strings.Join(Formats, ", "))
	}
}

// problem is one failed check or report issue, flattened for annotations and
// the Markdown summary.
type problem struct {
	status, collection, item, requirement, path, detail string
	file                                                string // path relative to the scan root to point an annotation at, if any
}

func problems(r Report) []problem {
	var out []problem
	kinds := map[string]string{}
	for _, g := range r.Collections {
		kinds[g.Name] = g.Kind
	}
	for _, i := range r.Issues {
		p := problem{status: i.Status, collection: i.Collection, path: i.Path, detail: i.Detail}
		if i.Status == "unlisted_item" && kinds[i.Collection] == "file" {
			p.file = i.Path
		}
		out = append(out, p)
	}
	for _, g := range r.Collections {
		for _, it := range g.Items {
			for _, c := range it.Checks {
				if c.Status == "present" {
					continue
				}
				p := problem{status: c.Status, collection: g.Name, item: it.Path, requirement: c.Name, path: c.Path, detail: c.Detail}
				switch {
				case c.Status == "too_small" || c.Status == "content_mismatch" || c.Status == "content_too_large" || c.Status == "unreadable":
					p.file = c.Path
				case g.Kind == "file":
					p.file = it.Path
				}
				out = append(out, p)
			}
		}
	}
	return out
}

// Workflow command escaping, as GitHub's toolkit does it.
func ghData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}
func ghProp(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

// renderGitHub prints one ::error annotation per problem. When the scan root
// is inside workspace, annotations name the file so they show up beside it.
func renderGitHub(w io.Writer, r Report, workspace string) error {
	prefix := ""
	if workspace != "" {
		if ws, err := filepath.EvalSymlinks(workspace); err == nil {
			if rel, err := filepath.Rel(ws, r.Root); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				prefix = filepath.ToSlash(rel)
			}
		}
	}
	if _, err := fmt.Fprintf(w, "UnGathered %s: %s. %d items, %d complete. %d checks: %d present, %d gaps, %d errors.\n", r.ToolVersion, r.Status, r.Totals.Items, r.Totals.CompleteItems, r.Totals.Checks, r.Totals.Present, r.Totals.Gaps, r.Totals.Errors); err != nil {
		return err
	}
	for _, p := range problems(r) {
		props := "title=" + ghProp("UnGathered "+p.status)
		if p.file != "" && prefix != "" {
			f := p.file
			if prefix != "." {
				f = prefix + "/" + f
			}
			props = "file=" + ghProp(f) + "," + props
		}
		msg := p.collection
		if p.item != "" {
			msg += " / " + p.item + " / " + p.requirement
		}
		msg += ": " + p.path
		if p.detail != "" {
			msg += " (" + p.detail + ")"
		}
		if _, err := fmt.Fprintf(w, "::error %s::%s\n", props, ghData(msg)); err != nil {
			return err
		}
	}
	return nil
}

// MaxSummaryRows caps the problem table in the Markdown summary; GitHub
// limits a job summary to 1 MiB.
const MaxSummaryRows = 200

func mdCell(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	var b strings.Builder
	for _, ch := range s {
		if strings.ContainsRune("\\`*_[]<>|#&!", ch) {
			b.WriteByte('\\')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func renderMarkdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "### UnGathered %s: %s\n\n", mdCell(r.ToolVersion), r.Status)
	b.WriteString("| Items | Complete | Checks | Present | Gaps | Errors |\n|---|---|---|---|---|---|\n")
	fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d |\n\n", r.Totals.Items, r.Totals.CompleteItems, r.Totals.Checks, r.Totals.Present, r.Totals.Gaps, r.Totals.Errors)
	for _, g := range r.Collections {
		if g.Roster != nil {
			fmt.Fprintf(&b, "Roster %s for %s: %d entries, %d found, %d missing, %d unlisted.\n\n", mdCell(g.Roster.File), mdCell(g.Name), g.Roster.Entries, g.Roster.Found, g.Roster.Missing, g.Roster.Unlisted)
		}
	}
	ps := problems(r)
	if len(ps) == 0 {
		b.WriteString("Every selected item has its declared companions.\n")
	} else {
		b.WriteString("| Problem | Collection | Item | Requirement | Path | Detail |\n|---|---|---|---|---|---|\n")
		for i, p := range ps {
			if i == MaxSummaryRows {
				fmt.Fprintf(&b, "\n%d more problems are in the full report.\n", len(ps)-MaxSummaryRows)
				break
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", p.status, mdCell(p.collection), mdCell(p.item), mdCell(p.requirement), mdCell(p.path), mdCell(p.detail))
		}
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

var htmlReport = template.Must(template.New("report").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>UnGathered collection report</title><style>body{font:16px system-ui;max-width:1100px;margin:2rem auto;padding:0 1rem;background:#f5f7fa;color:#182b3e}p{line-height:1.6}table{width:100%;border-collapse:collapse;margin:1rem 0;background:white}td,th{padding:10px;border:1px solid #cbd4df;text-align:left;overflow-wrap:anywhere}code{overflow-wrap:anywhere}.present{color:#17613f}.missing,.too_small,.content_mismatch,.wrong_type{color:#9b4415}.unreadable,.unsafe_symlink,.unsafe_path,.content_too_large{color:#a12239}.scroll{overflow-x:auto}</style><h1>UnGathered collection report</h1><p>Version {{.ToolVersion}}. Status: {{.Status}}. Root: <code>{{.Root}}</code></p><p>{{.Totals.Items}} items, {{.Totals.CompleteItems}} complete. {{.Totals.Checks}} checks: {{.Totals.Present}} present, {{.Totals.Gaps}} gaps, {{.Totals.Errors}} operational errors.</p>{{if .Issues}}<h2>Collection and scan issues</h2><ul>{{range .Issues}}<li>{{.Collection}}: {{.Status}} at <code>{{.Path}}</code>. {{.Detail}}</li>{{end}}</ul>{{end}}{{range .Collections}}<h2>{{.Name}}</h2><p>Anchor pattern: <code>{{.Pattern}}</code>{{range .Exclude}}, excluding <code>{{.}}</code>{{end}}. {{len .Items}} selected items.</p>{{with .Roster}}<p>Roster <code>{{.File}}</code> (match by {{.Match}}): {{.Entries}} entries, {{.Found}} found, {{.Missing}} missing, {{.Unlisted}} unlisted.</p>{{end}}{{range .Items}}<h3>{{.Path}}</h3><p>Complete: {{.Complete}}</p><div class="scroll"><table><tr><th>Requirement</th><th>Expected path</th><th>Result</th><th>Detail</th></tr>{{range .Checks}}<tr><td>{{.Name}}</td><td><code>{{.Path}}</code></td><td class="{{.Status}}">{{.Status}}</td><td>{{.Detail}}</td></tr>{{end}}</table></div>{{end}}{{end}}<p>Manifest SHA-256: <code>{{.ManifestSHA256}}</code></p><p>Completeness applies only to selected items and declared rules. Metadata and content were read during this scan, without an atomic filesystem snapshot. No network requests, file repairs or deletions were performed.</p></html>`))
