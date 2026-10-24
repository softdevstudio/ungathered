package scan

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
)

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
	default:
		return fmt.Errorf("format must be text, json or html")
	}
}

var htmlReport = template.Must(template.New("report").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>UnGathered collection report</title><style>body{font:16px system-ui;max-width:1100px;margin:2rem auto;padding:0 1rem;background:#f5f7fa;color:#182b3e}p{line-height:1.6}table{width:100%;border-collapse:collapse;margin:1rem 0;background:white}td,th{padding:10px;border:1px solid #cbd4df;text-align:left;overflow-wrap:anywhere}code{overflow-wrap:anywhere}.present{color:#17613f}.missing,.too_small,.content_mismatch,.wrong_type{color:#9b4415}.unreadable,.unsafe_symlink,.unsafe_path,.content_too_large{color:#a12239}.scroll{overflow-x:auto}</style><h1>UnGathered collection report</h1><p>Version {{.ToolVersion}}. Status: {{.Status}}. Root: <code>{{.Root}}</code></p><p>{{.Totals.Items}} items, {{.Totals.CompleteItems}} complete. {{.Totals.Checks}} checks: {{.Totals.Present}} present, {{.Totals.Gaps}} gaps, {{.Totals.Errors}} operational errors.</p>{{if .Issues}}<h2>Collection and scan issues</h2><ul>{{range .Issues}}<li>{{.Collection}}: {{.Status}} at <code>{{.Path}}</code>. {{.Detail}}</li>{{end}}</ul>{{end}}{{range .Collections}}<h2>{{.Name}}</h2><p>Anchor pattern: <code>{{.Pattern}}</code>. {{len .Items}} selected items.</p>{{range .Items}}<h3>{{.Path}}</h3><p>Complete: {{.Complete}}</p><div class="scroll"><table><tr><th>Requirement</th><th>Expected path</th><th>Result</th><th>Detail</th></tr>{{range .Checks}}<tr><td>{{.Name}}</td><td><code>{{.Path}}</code></td><td class="{{.Status}}">{{.Status}}</td><td>{{.Detail}}</td></tr>{{end}}</table></div>{{end}}{{end}}<p>Manifest SHA-256: <code>{{.ManifestSHA256}}</code></p><p>Completeness applies only to selected items and declared rules. Metadata and content were read during this scan, without an atomic filesystem snapshot. No network requests, file repairs or deletions were performed.</p></html>`))
