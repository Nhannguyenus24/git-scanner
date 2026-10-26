package scan

import (
	"path"
	"strings"
)

// FileKind classifies the file a finding was found in, for the question an
// analyst asks first: where do credentials leak? It is derived from the path
// alone, at read time, so it applies to every finding ever stored and a change
// to the categories needs no migration.
//
// The order of the checks is the answer to "which kind wins": third-party code
// first, since a test or config file inside a committed virtualenv is still
// somebody else's package; then a template or a fixture, whatever its
// extension, because its values were meant to be replaced or were never meant
// to be real; then files whose only job is to hold secrets; then where the
// file sits in the delivery pipeline; and only then the general format.
func FileKind(p string) string {
	lower := strings.ToLower(p)
	base := path.Base(lower)
	ext := path.Ext(base)

	switch {
	case dependencyPath(lower):
		return "dependency"
	case exampleFile(p):
		return "example"
	case isTestPath(p):
		return "test"
	case credentialFile(base, ext):
		return "credential_file"
	case base == ".env" || strings.HasPrefix(base, ".env.") || ext == ".env":
		return "env"
	case ciFile(lower, base):
		return "ci"
	case base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") ||
		strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "compose."):
		return "container"
	case ext == ".tf" || ext == ".tfvars" || ext == ".hcl" ||
		strings.Contains(lower, "/k8s/") || strings.Contains(lower, "/helm/") || strings.HasPrefix(lower, "k8s/"):
		return "infra"
	case ext == ".ipynb":
		return "notebook"
	}

	switch ext {
	case ".yml", ".yaml", ".json", ".toml", ".ini", ".cfg", ".conf", ".properties", ".xml", ".plist", ".config":
		return "config"
	case ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd":
		return "script"
	case ".md", ".rst", ".txt", ".adoc", ".html", ".htm":
		return "docs"
	case ".sql", ".csv", ".log", ".dump", ".bak", ".jsonl", ".ndjson":
		return "data"
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".java", ".kt", ".kts", ".scala",
		".rb", ".php", ".cs", ".swift", ".m", ".c", ".h", ".cc", ".cpp", ".hpp", ".rs", ".dart",
		".vue", ".svelte", ".lua", ".r", ".pl", ".groovy", ".gradle", ".ex", ".exs", ".erl", ".clj":
		return "source"
	}
	return "other"
}

// dependencyPath reports third-party code committed into the repository: a
// virtualenv, node_modules, a vendor tree, package metadata. On the random
// corpus this was the largest single source of findings — a committed
// virtualenv's dist-info/RECORD files are lists of sha256 file hashes, which
// the generic pass reads as secrets.
func dependencyPath(lower string) bool {
	if strings.Contains(lower, ".dist-info/") || strings.Contains(lower, ".egg-info/") {
		return true
	}
	for _, seg := range strings.Split(lower, "/") {
		switch seg {
		case "node_modules", "vendor", "site-packages", "dist-packages", "bower_components",
			".venv", "venv", "virtualenv", "third_party", "third-party", "pods", ".bundle":
			return true
		}
	}
	return false
}

// credentialFile reports files that exist to hold a credential: key material,
// tool credential stores, cloud service-account keys.
func credentialFile(base, ext string) bool {
	switch ext {
	case ".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".ppk":
		return true
	}
	switch base {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", ".npmrc", ".pypirc", ".netrc", ".git-credentials",
		".htpasswd", ".dockercfg", "credentials", "credentials.json", "client_secret.json", "secrets.json",
		"secrets.yml", "secrets.yaml", "service-account.json", "serviceaccount.json", ".s3cfg", ".boto":
		return true
	}
	return strings.HasPrefix(base, "client_secret_") && ext == ".json"
}

// ciFile reports a pipeline definition: secrets there are usually meant to be
// injected by the CI system and were pasted in literally instead.
func ciFile(lower, base string) bool {
	if strings.Contains(lower, ".github/workflows/") || strings.Contains(lower, ".circleci/") ||
		strings.Contains(lower, ".buildkite/") {
		return true
	}
	switch base {
	case ".gitlab-ci.yml", "jenkinsfile", ".travis.yml", "azure-pipelines.yml", "bitbucket-pipelines.yml",
		"cloudbuild.yaml", "cloudbuild.yml", "buildspec.yml", ".drone.yml", "appveyor.yml":
		return true
	}
	return false
}
