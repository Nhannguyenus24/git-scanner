package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"path"
	"sort"
	"strings"

	"github.com/mservice/gitscanner/libs/entity"
)

// RenderDisclosure builds the disclosure mail for a repository, as plain text
// and as HTML from the same view, so the two parts cannot say different things.
//
// The mail carries what the owner needs to verify the report and act on it: the
// masked preview, a link to the exact line at the exact commit, the enricher's
// provider and liveness facts, and removal steps written for the files actually
// involved. A notice the reader can check for themselves is one they act on; a
// vague one reads as phishing. It never contains the secret itself, and per
// plan.md §0.3 it never contains a payment link, a bounty request or any other
// ask — the mail exists to get the credential rotated, and anything that reads
// as leverage would undermine that.
func RenderDisclosure(repo entity.Repository, findings []entity.Finding, info DisclosureInfo) Message {
	v := buildView(repo, findings, info)
	var html bytes.Buffer
	if err := htmlTmpl.Execute(&html, v); err != nil {
		// The template is static and the view is plain data, so this is a
		// programming error; the text part still goes out on its own.
		html.Reset()
	}
	return Message{
		Subject: fmt.Sprintf("Credentials found in public repository %s", repo.FullName()),
		Body:    renderText(v),
		HTML:    html.String(),
	}
}

type disclosureView struct {
	Repo      string
	RepoURL   string
	Project   string
	Count     string // "3 values"
	Live      int
	Findings  []findingView
	Guide     []guideStep
	InfoURL   string
	Signature string
}

type findingView struct {
	Title    string // provider name, or the rule when unattributed
	Location string // path:line
	Masked   string
	Severity string
	Live     bool
	Details  []detail
	Link     string
	OnHead   bool
}

type detail struct{ Key, Value string }

type guideStep struct {
	Title    string
	Text     []string
	Commands []string
}

func buildView(repo entity.Repository, findings []entity.Finding, info DisclosureInfo) disclosureView {
	v := disclosureView{
		Repo:      repo.FullName(),
		RepoURL:   "https://github.com/" + repo.FullName(),
		Project:   info.Project,
		Count:     plural(len(findings), "value", "values"),
		Guide:     removalGuide(repo, findings),
		InfoURL:   info.InfoURL,
		Signature: info.Signature,
	}
	if v.Project == "" {
		v.Project = "an automated secret-scanning project"
	}
	for _, f := range sortedBySeverity(findings) {
		fv := findingView{
			Title:    providerName(f),
			Location: f.FilePath,
			Masked:   f.MaskedPreview,
			Severity: f.Severity.String(),
			Live:     f.Metadata["probe_status"] == "live",
			Details:  details(f),
			Link:     blobURL(repo, f),
			OnHead:   f.ReachableFromHEAD,
		}
		if f.LineNumber > 0 {
			fv.Location += fmt.Sprintf(":%d", f.LineNumber)
		}
		if fv.Live {
			v.Live++
		}
		v.Findings = append(v.Findings, fv)
	}
	return v
}

func providerName(f entity.Finding) string {
	if n := f.Metadata["provider_name"]; n != "" {
		return n
	}
	if f.ProviderSlug != "" {
		return f.ProviderSlug
	}
	return "Unrecognised credential"
}

// details are the non-secret facts the scan and the enricher gathered, in the
// order a reader weighs them: what it is, whether it still works, whose it is,
// and where it came from.
func details(f entity.Finding) []detail {
	var out []detail
	add := func(k, v string) {
		if v != "" {
			out = append(out, detail{k, v})
		}
	}
	if f.Metadata["provider_name"] != "" || f.ProviderSlug != "" {
		add("provider", providerName(f))
	}
	add("category", f.Metadata["category"])
	switch f.Metadata["probe_status"] {
	case "live":
		add("status", "CONFIRMED LIVE by a check against the provider -- rotate this first")
	case "dead":
		add("status", "the provider rejected it (already rotated or revoked)")
	}
	if id := f.Metadata["probe_owner"]; id != "" {
		add("account", "authenticates as "+id)
	} else {
		add("account", f.Metadata["probe_account"])
	}
	add("aws acct", f.Metadata["aws_account_id"])
	if f.CommitSHA != "" {
		c := shortSHA(f.CommitSHA)
		if f.AuthorName != "" {
			c += " by " + f.AuthorName
		}
		if !f.CommittedAt.IsZero() {
			c += " on " + f.CommittedAt.UTC().Format("2006-01-02")
		}
		add("commit", c)
	}
	add("rule", f.RuleID)
	return out
}

func renderText(v disclosureView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Hello,\n\n")
	fmt.Fprintf(&b, "I run %s. A scan of the public repository %s found %s that look like "+
		"live credentials committed to its history. I am writing so you can rotate them "+
		"before someone else finds and abuses them.\n\n", v.Project, v.Repo, v.Count)

	b.WriteString("What was found:\n\n")
	for _, f := range v.Findings {
		fmt.Fprintf(&b, "  - %s\n", f.Location)
		fmt.Fprintf(&b, "    %-9s %s\n", "value:", f.Masked)
		fmt.Fprintf(&b, "    %-9s %s\n", "severity:", f.Severity)
		for _, d := range f.Details {
			fmt.Fprintf(&b, "    %-9s %s\n", d.Key+":", d.Value)
		}
		if f.Link != "" {
			fmt.Fprintf(&b, "    %-9s %s\n", "link:", f.Link)
		}
		fmt.Fprintf(&b, "    %-9s %s\n\n", "note:", headNote(f.OnHead))
	}

	b.WriteString(verifyText + "\n\n")
	b.WriteString("What I suggest:\n\n")
	for i, s := range v.Guide {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, s.Title)
		for _, t := range s.Text {
			fmt.Fprintf(&b, "     %s\n", t)
		}
		if len(s.Commands) > 0 {
			b.WriteString("\n")
			for _, c := range s.Commands {
				fmt.Fprintf(&b, "       %s\n", c)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(optOutText + "\n")
	if v.InfoURL != "" {
		fmt.Fprintf(&b, "\nMore about the project: %s\n", v.InfoURL)
	}
	if v.Signature != "" {
		fmt.Fprintf(&b, "\n%s\n", v.Signature)
	}
	return b.String()
}

const (
	verifyText = "How to check this report yourself: open the links above -- each points at " +
		"the commit that introduced the value, so you can confirm it without trusting this " +
		"mail. Only a masked form of each value is shown; the full values are not stored, " +
		"shared or included here. This mail contains no attachments and asks nothing of you " +
		"beyond rotating the credentials."
	optOutText = "If these are test or placeholder values, or you would rather not receive " +
		"this kind of notice, reply and I will suppress future mail for your address or domain."
)

func headNote(onHead bool) string {
	if onHead {
		return "still present on the default branch"
	}
	return "not on the default branch, but still present in history"
}

// removalGuide gives steps for the files actually involved. An env file is
// dropped from history whole; a source file must keep its other contents, so
// only the values are rewritten. Rewriting main.go out of existence would be
// worse advice than none.
func removalGuide(repo entity.Repository, findings []entity.Finding) []guideStep {
	var envFiles, sourceFiles []string
	seen := map[string]bool{}
	for _, f := range findings {
		if f.FilePath == "" || seen[f.FilePath] {
			continue
		}
		seen[f.FilePath] = true
		if isEnvFile(f.FilePath) {
			envFiles = append(envFiles, f.FilePath)
		} else {
			sourceFiles = append(sourceFiles, f.FilePath)
		}
	}
	sort.Strings(envFiles)
	sort.Strings(sourceFiles)
	url := cloneURL(repo)

	steps := []guideStep{
		{
			Title: "Rotate the affected credentials at the provider.",
			Text: []string{"This is the step that matters. A credential in public git history " +
				"should be treated as compromised even after the file is changed: rewriting " +
				"history hides it from new clones, but anyone who already fetched it keeps it."},
		},
		{
			Title: "Work in a fresh clone.",
			Text: []string{"git-filter-repo (https://github.com/newren/git-filter-repo) " +
				"refuses to rewrite a clone with local changes."},
			Commands: []string{fmt.Sprintf("git clone %s && cd %s", url, repo.Name)},
		},
	}

	rewrite := guideStep{Title: "Rewrite the values out of history."}
	if len(envFiles) > 0 {
		cmd := "git filter-repo --invert-paths"
		for _, p := range envFiles {
			cmd += " --path " + p
		}
		rewrite.Text = append(rewrite.Text, "Env files hold nothing but configuration, so drop them from every commit:")
		rewrite.Commands = append(rewrite.Commands, cmd)
	}
	if len(sourceFiles) > 0 {
		rewrite.Text = append(rewrite.Text, "Other files keep their contents; only the values are "+
			"replaced. Put each leaked value on its own line of a file called replacements.txt, "+
			"followed by ==>REMOVED, and run the command below.",
			"Files involved: "+strings.Join(sourceFiles, ", "))
		rewrite.Commands = append(rewrite.Commands, "git filter-repo --replace-text replacements.txt")
	}
	steps = append(steps, rewrite)

	keep := guideStep{Title: "Keep the next value out of git."}
	if len(envFiles) > 0 {
		for _, p := range envFiles {
			keep.Commands = append(keep.Commands, fmt.Sprintf("echo '%s' >> .gitignore", p))
		}
		keep.Commands = append(keep.Commands, "git add .gitignore && git commit -m 'Ignore env files'")
		keep.Text = append(keep.Text, "Commit a copy with placeholder values instead (for example "+
			path.Base(envFiles[0])+".example) so collaborators know which variables to set.")
	}
	if len(sourceFiles) > 0 {
		keep.Text = append(keep.Text, "Read credentials from environment variables or a secret "+
			"manager rather than writing them into source or config files.")
	}
	steps = append(steps, keep,
		guideStep{
			Title:    "Publish the rewritten history.",
			Text:     []string{"filter-repo removes the remote on purpose, so add it back before pushing."},
			Commands: []string{"git remote add origin " + url, "git push --force --all && git push --force --tags"},
		},
		guideStep{
			Title: "Make sure the old history does not come back.",
			Text: []string{"Collaborators should re-clone rather than pull, or an old clone will " +
				"push the values back. Copies can survive in forks, pull request refs and " +
				"GitHub's cached views; GitHub's guide covers asking support to purge them: " +
				"https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository"},
		},
	)
	return steps
}

// isEnvFile reports whether a path is a dotenv-style or credentials file whose
// whole content is configuration, so dropping it from history loses nothing.
func isEnvFile(p string) bool {
	base := strings.ToLower(path.Base(p))
	return base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env") ||
		base == ".npmrc" || base == ".pypirc" || base == ".netrc"
}

// blobURL links to the line at the commit that introduced it. Only GitHub URLs
// are built; for another host the commit id alone is enough to find it.
func blobURL(repo entity.Repository, f entity.Finding) string {
	if f.CommitSHA == "" || (repo.CloneURL != "" && !strings.Contains(repo.CloneURL, "github.com")) {
		return ""
	}
	u := fmt.Sprintf("https://github.com/%s/blob/%s/%s", repo.FullName(), f.CommitSHA, f.FilePath)
	if f.LineNumber > 0 {
		u += fmt.Sprintf("#L%d", f.LineNumber)
	}
	return u
}

func cloneURL(repo entity.Repository) string {
	if repo.CloneURL != "" {
		return repo.CloneURL
	}
	return "https://github.com/" + repo.FullName() + ".git"
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// sortedBySeverity orders findings so the mail leads with what to rotate
// first: credentials a probe confirmed live, then the most severe, without
// mutating the caller's slice. A live test key outranks a dead production one.
func sortedBySeverity(findings []entity.Finding) []entity.Finding {
	out := make([]entity.Finding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := out[i].Metadata["probe_status"] == "live", out[j].Metadata["probe_status"] == "live"
		if li != lj {
			return li
		}
		return out[i].Severity > out[j].Severity
	})
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// sevStyle is the badge colour per severity: background and text, chosen for
// contrast on white since mail clients ignore dark-mode tokens.
var sevStyle = map[string][2]string{
	"critical": {"#FEE4E2", "#B42318"},
	"high":     {"#FFEAD5", "#C4320A"},
	"medium":   {"#FEF0C7", "#B54708"},
	"low":      {"#F2F4F7", "#475467"},
	"info":     {"#F2F4F7", "#475467"},
}

// Mail clients strip <style> blocks and ignore most CSS, so the layout is
// tables with inline styles — the one form Gmail, Outlook and Apple Mail all
// render the same way.
var htmlTmpl = template.Must(template.New("disclosure").Funcs(template.FuncMap{
	"sevBg":    func(s string) string { return sevStyle[s][0] },
	"sevFg":    func(s string) string { return sevStyle[s][1] },
	"inc":      func(i int) int { return i + 1 },
	"headNote": headNote,
}).Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Security notice</title></head>
<body style="margin:0;padding:0;background:#F4F5F7;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#F4F5F7;">
<tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:640px;background:#FFFFFF;border:1px solid #E4E7EC;border-radius:12px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#101828;">

<tr><td style="padding:28px 32px 20px;border-bottom:1px solid #EAECF0;">
  <div style="font-size:12px;font-weight:700;letter-spacing:.08em;color:#B42318;text-transform:uppercase;">Security notice</div>
  <div style="font-size:22px;font-weight:700;line-height:1.3;margin-top:6px;">Credentials exposed in <a href="{{.RepoURL}}" style="color:#101828;">{{.Repo}}</a></div>
  <div style="margin-top:12px;">
    <span style="display:inline-block;padding:4px 10px;border-radius:999px;background:#F2F4F7;color:#344054;font-size:13px;font-weight:600;">{{.Count}} found</span>
    {{if .Live}}<span style="display:inline-block;padding:4px 10px;border-radius:999px;background:#FEE4E2;color:#B42318;font-size:13px;font-weight:600;margin-left:6px;">{{.Live}} confirmed live</span>{{end}}
  </div>
</td></tr>

<tr><td style="padding:24px 32px 8px;font-size:15px;line-height:1.6;color:#344054;">
  <p style="margin:0 0 12px;">Hello,</p>
  <p style="margin:0;">I run {{.Project}}. A scan of the public repository <strong>{{.Repo}}</strong> found {{.Count}} that look like live credentials committed to its history. I am writing so you can rotate them before someone else finds and abuses them.</p>
</td></tr>

<tr><td style="padding:16px 32px 0;">
  <div style="font-size:13px;font-weight:700;letter-spacing:.06em;color:#667085;text-transform:uppercase;margin-bottom:10px;">What was found</div>
  {{range .Findings}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border:1px solid {{if .Live}}#FDA29B{{else}}#EAECF0{{end}};border-radius:10px;margin-bottom:12px;">
  <tr><td style="padding:16px 18px;">
    <span style="display:inline-block;padding:2px 8px;border-radius:6px;background:{{sevBg .Severity}};color:{{sevFg .Severity}};font-size:12px;font-weight:700;text-transform:uppercase;">{{.Severity}}</span>
    {{if .Live}}<span style="display:inline-block;padding:2px 8px;border-radius:6px;background:#B42318;color:#FFFFFF;font-size:12px;font-weight:700;margin-left:4px;">LIVE &middot; rotate first</span>{{end}}
    <div style="font-size:16px;font-weight:700;margin-top:8px;">{{.Title}}</div>
    <div style="font-family:SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;color:#475467;margin-top:4px;word-break:break-all;">{{.Location}}</div>
    <div style="margin-top:10px;"><span style="font-family:SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;background:#F2F4F7;border-radius:6px;padding:4px 8px;color:#101828;">{{.Masked}}</span></div>
    <table role="presentation" cellpadding="0" cellspacing="0" style="margin-top:12px;font-size:13px;line-height:1.5;">
      {{range .Details}}{{if ne .Key "provider"}}<tr><td style="color:#667085;padding:2px 14px 2px 0;white-space:nowrap;vertical-align:top;">{{.Key}}</td><td style="color:#101828;padding:2px 0;">{{.Value}}</td></tr>{{end}}{{end}}
      <tr><td style="color:#667085;padding:2px 14px 2px 0;white-space:nowrap;vertical-align:top;">note</td><td style="color:#101828;padding:2px 0;">{{headNote .OnHead}}</td></tr>
    </table>
    {{if .Link}}<div style="margin-top:14px;"><a href="{{.Link}}" style="display:inline-block;padding:8px 14px;border-radius:8px;background:#101828;color:#FFFFFF;font-size:13px;font-weight:600;text-decoration:none;">View the line on GitHub &rarr;</a></div>{{end}}
  </td></tr>
  </table>
  {{end}}
</td></tr>

<tr><td style="padding:4px 32px 8px;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#EFF8FF;border-radius:10px;">
  <tr><td style="padding:14px 18px;font-size:14px;line-height:1.55;color:#175CD3;">
    <strong>Check this report yourself.</strong> Each link opens the commit that introduced the value, so you can confirm it without trusting this mail. Only a masked form of each value is shown; the full values are not stored, shared or included here. This mail has no attachments and asks nothing of you beyond rotating the credentials.
  </td></tr>
  </table>
</td></tr>

<tr><td style="padding:20px 32px 4px;">
  <div style="font-size:13px;font-weight:700;letter-spacing:.06em;color:#667085;text-transform:uppercase;margin-bottom:6px;">What to do</div>
  {{range $i, $s := .Guide}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin-top:12px;">
  <tr>
    <td width="32" valign="top" style="padding-top:1px;"><div style="width:24px;height:24px;border-radius:12px;background:{{if eq $i 0}}#B42318{{else}}#101828{{end}};color:#FFFFFF;font-size:13px;font-weight:700;text-align:center;line-height:24px;">{{inc $i}}</div></td>
    <td valign="top" style="font-size:14px;line-height:1.55;color:#344054;">
      <div style="font-weight:700;color:#101828;font-size:15px;">{{$s.Title}}</div>
      {{range $s.Text}}<div style="margin-top:4px;">{{.}}</div>{{end}}
      {{if $s.Commands}}<div style="margin-top:8px;background:#101828;border-radius:8px;padding:12px 14px;font-family:SFMono-Regular,Menlo,Consolas,monospace;font-size:12.5px;line-height:1.7;color:#E4E7EC;word-break:break-all;">{{range $s.Commands}}<div>{{.}}</div>{{end}}</div>{{end}}
    </td>
  </tr>
  </table>
  {{end}}
</td></tr>

<tr><td style="padding:24px 32px 28px;font-size:13px;line-height:1.6;color:#667085;border-top:1px solid #EAECF0;margin-top:16px;">
  <p style="margin:0 0 8px;">If these are test or placeholder values, or you would rather not receive this kind of notice, reply and I will suppress future mail for your address or domain.</p>
  {{if .InfoURL}}<p style="margin:0 0 8px;">More about the project: <a href="{{.InfoURL}}" style="color:#175CD3;">{{.InfoURL}}</a></p>{{end}}
  {{if .Signature}}<p style="margin:12px 0 0;color:#344054;">{{.Signature}}</p>{{end}}
</td></tr>

</table>
</td></tr>
</table>
</body></html>
`))
