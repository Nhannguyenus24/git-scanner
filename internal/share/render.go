package share

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// Markdown renders the report for pasting into an issue, a ticket or a chat.
func Markdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Secret scan report: %s\n\n", r.Repo)
	fmt.Fprintf(&b, "Repository: %s  \n", r.RepoURL)
	if r.LastScannedAt != nil {
		fmt.Fprintf(&b, "Last scanned: %s  \n", r.LastScannedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	fmt.Fprintf(&b, "Report generated: %s  \n", r.GeneratedAt.Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "Link valid until: %s\n\n", r.ExpiresAt.Format("2006-01-02 15:04 UTC"))
	b.WriteString("> " + notice(r) + "\n\n")

	s := r.Summary
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Findings | %d |\n| Open | %d |\n| Confirmed live | %d |\n| Still on the default branch | %d |\n| Files involved | %d |\n\n",
		s.Total, s.Open, s.Live, s.OnHead, s.Files)
	for _, sec := range []struct {
		title string
		rows  []Count
	}{{"By severity", s.BySeverity}, {"By provider", s.ByProvider}, {"By status", s.ByStatus}} {
		if len(sec.rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n| | Count |\n|---|---:|\n", sec.title)
		for _, c := range sec.rows {
			fmt.Fprintf(&b, "| %s | %d |\n", cell(c.Key), c.Count)
		}
		b.WriteString("\n")
	}

	if len(r.Findings) > 0 {
		b.WriteString("## Findings\n\n| Severity | Provider | Location | Value (masked) | Commit | Status |\n|---|---|---|---|---|---|\n")
		for _, f := range r.Findings {
			commit := f.Commit
			if f.CommittedAt != "" {
				commit += " (" + f.CommittedAt + ")"
			}
			if f.Link != "" {
				commit = "[" + cell(commit) + "](" + f.Link + ")"
			} else {
				commit = cell(commit)
			}
			sev := f.Severity
			if f.Live {
				sev += " · LIVE"
			}
			loc := "`" + strings.ReplaceAll(f.Location, "`", "'") + "`"
			if !f.OnHead {
				loc += " (history only)"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | `%s` | %s | %s |\n",
				cell(sev), cell(f.Provider), cell(loc), strings.ReplaceAll(f.Masked, "`", "'"), commit, cell(f.Status))
		}
		b.WriteString("\n")
	}

	if len(r.Guide) > 0 {
		b.WriteString("## How to fix it\n\n")
		for i, g := range r.Guide {
			fmt.Fprintf(&b, "%d. **%s**\n", i+1, g.Title)
			for _, t := range g.Text {
				fmt.Fprintf(&b, "   %s\n", t)
			}
			if len(g.Commands) > 0 {
				b.WriteString("\n   ```sh\n")
				for _, c := range g.Commands {
					fmt.Fprintf(&b, "   %s\n", c)
				}
				b.WriteString("   ```\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// cell keeps a value from breaking a Markdown table row.
func cell(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").Replace(s)
}

func notice(r Report) string {
	base := "Only a masked form of each value is shown; the values themselves are not stored and are not in this report."
	if r.Detail == "full" {
		return "This report shows where each credential sits in the repository's history. Treat it as sensitive and share it only with whoever will rotate the credentials. " + base
	}
	return "This is a summary: it says how many credentials were found and of what kind, not where they are. " + base
}

// HTML renders a self-contained page meant to be read in a browser or printed
// to PDF. It loads nothing from anywhere, so opening it does not tell a third
// party the report was opened.
func HTML(r Report) (string, error) {
	var b bytes.Buffer
	err := htmlTmpl.Execute(&b, struct {
		Report
		Notice string
	}{r, notice(r)})
	return b.String(), err
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"date": func(t interface{ Format(string) string }) string { return t.Format("2006-01-02 15:04 UTC") },
	"inc":  func(i int) int { return i + 1 },
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer">
<meta name="robots" content="noindex,nofollow">
<title>Secret scan report: {{.Repo}}</title>
<style>
:root{--fg:#101828;--muted:#475467;--line:#EAECF0;--bg:#FFFFFF;--soft:#F9FAFB}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
main{max-width:960px;margin:0 auto;padding:32px 16px 48px}
h1{font-size:24px;margin:0 0 4px}h2{font-size:17px;margin:32px 0 10px;padding-bottom:6px;border-bottom:1px solid var(--line)}
.meta{color:var(--muted);font-size:13px}
.notice{margin:16px 0;padding:10px 14px;border-left:3px solid #B54708;background:#FFFAEB;color:#7A2E0E;font-size:13px}
.kpis{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px}
.kpi{border:1px solid var(--line);border-radius:8px;padding:10px 12px}.kpi b{display:block;font-size:22px}
.kpi span{color:var(--muted);font-size:12px}
.cols{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:16px}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid var(--line);vertical-align:top}
th{color:var(--muted);font-weight:600;font-size:12px}
td.n{text-align:right;font-variant-numeric:tabular-nums}
code,pre{font-family:"IBM Plex Mono",ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}
td code{word-break:break-all}
pre{background:var(--soft);border:1px solid var(--line);border-radius:6px;padding:8px 10px;white-space:pre-wrap;word-break:break-all}
.sev{display:inline-block;padding:1px 8px;border-radius:999px;font-size:11px;font-weight:700;text-transform:uppercase}
.critical{background:#FEE4E2;color:#B42318}.high{background:#FFEAD5;color:#C4320A}.medium{background:#FEF0C7;color:#B54708}.low,.info{background:#F2F4F7;color:#475467}
.live{color:#B42318;font-weight:700;font-size:11px}
.scroll{overflow-x:auto}
ol.guide li{margin-bottom:12px}
footer{margin-top:40px;color:var(--muted);font-size:12px}
@media print{main{padding:0}a{color:inherit}.scroll{overflow:visible}h2{break-after:avoid}tr{break-inside:avoid}}
</style></head>
<body><main>
<h1>Secret scan report: {{.Repo}}</h1>
<div class="meta"><a href="{{.RepoURL}}" rel="noreferrer noopener">{{.RepoURL}}</a>
{{- if .LastScannedAt}} · last scanned {{date .LastScannedAt}}{{end}} · generated {{date .GeneratedAt}} · link valid until {{date .ExpiresAt}}</div>
<div class="notice">{{.Notice}}</div>

<h2>Summary</h2>
<div class="kpis">
<div class="kpi"><b>{{.Summary.Total}}</b><span>findings</span></div>
<div class="kpi"><b>{{.Summary.Open}}</b><span>open</span></div>
<div class="kpi"><b>{{.Summary.Live}}</b><span>confirmed live</span></div>
<div class="kpi"><b>{{.Summary.OnHead}}</b><span>still on the default branch</span></div>
<div class="kpi"><b>{{.Summary.Files}}</b><span>files involved</span></div>
</div>
<div class="cols">
{{with .Summary.BySeverity}}<div><h2>By severity</h2><table>{{range .}}<tr><td><span class="sev {{.Key}}">{{.Key}}</span></td><td class="n">{{.Count}}</td></tr>{{end}}</table></div>{{end}}
{{with .Summary.ByProvider}}<div><h2>By provider</h2><table>{{range .}}<tr><td>{{.Key}}</td><td class="n">{{.Count}}</td></tr>{{end}}</table></div>{{end}}
{{with .Summary.ByStatus}}<div><h2>By status</h2><table>{{range .}}<tr><td>{{.Key}}</td><td class="n">{{.Count}}</td></tr>{{end}}</table></div>{{end}}
</div>

{{if .Findings}}<h2>Findings</h2>
<div class="scroll"><table>
<tr><th>Severity</th><th>Provider</th><th>Location</th><th>Value (masked)</th><th>Commit</th><th>Status</th></tr>
{{range .Findings}}<tr>
<td><span class="sev {{.Severity}}">{{.Severity}}</span>{{if .Live}}<div class="live">LIVE</div>{{end}}</td>
<td>{{.Provider}}<div class="meta"><code>{{.Rule}}</code></div></td>
<td><code>{{.Location}}</code>{{if not .OnHead}}<div class="meta">history only</div>{{end}}</td>
<td><code>{{.Masked}}</code></td>
<td>{{if .Link}}<a href="{{.Link}}" rel="noreferrer noopener"><code>{{.Commit}}</code></a>{{else}}<code>{{.Commit}}</code>{{end}}{{if .CommittedAt}}<div class="meta">{{.CommittedAt}}</div>{{end}}</td>
<td>{{.Status}}</td>
</tr>{{end}}
</table></div>{{end}}

{{if .Guide}}<h2>How to fix it</h2>
<ol class="guide">{{range .Guide}}<li><strong>{{.Title}}</strong>{{range .Text}}<div>{{.}}</div>{{end}}{{if .Commands}}<pre>{{range .Commands}}{{.}}
{{end}}</pre>{{end}}</li>{{end}}</ol>{{end}}

<footer>Generated by GitScanner. This page loads nothing from the network.</footer>
</main></body></html>`))
