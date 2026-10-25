package scan

import (
	"bytes"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// Local-only detection is a false-positive filter (plan.md §4.1 step 6).
//
// A credential whose configuration points only at the developer's own machine
// (postgres://app:secret@localhost/app, DB_HOST=127.0.0.1) is a local
// development value: it opens nothing reachable from outside that machine, and
// reporting it trains people to ignore the tool. Such matches are skipped.
//
// Deliberately narrow: only loopback, the unspecified address and the
// container-to-host aliases count as local. Private ranges (10/8, 172.16/12,
// 192.168/16) and *.internal names are NOT local — they are real shared
// infrastructure, and a leaked credential for them is exactly what an
// attacker who gets onto that network uses next.

// localHostNames are names that always resolve to the machine running the
// code, or to the host of the container running it.
var localHostNames = map[string]bool{
	"localhost":                  true,
	"localhost.localdomain":      true,
	"ip6-localhost":              true,
	"ip6-loopback":               true,
	"host.docker.internal":       true,
	"gateway.docker.internal":    true,
	"kubernetes.docker.internal": true,
	"host.k3d.internal":          true,
	"host.containers.internal":   true, // podman
	"host.minikube.internal":     true,
}

// hostScopedProviders are the providers whose credential is a URL that names
// the one host it unlocks — a database, cache, broker or mail server. Only for
// these does a localhost URL mean a local-only credential; a provider-issued
// API key works from anywhere whatever host sits beside it (see LocalOnly).
var hostScopedProviders = map[string]bool{
	"postgres": true, "mysql": true, "mongodb": true, "redis": true,
	"rabbitmq": true, "mssql": true, "smtp": true,
}

// composeServiceNames are the single-label host names a credential points at
// when it names a service inside a docker-compose or Kubernetes network rather
// than a routable host (postgres://app:pw@db:5432, REDIS_HOST=redis). Such a
// name resolves only inside that orchestration network, so the credential it
// carries reaches nothing from outside — the same reason localhost is dropped.
//
// Only the exact bare labels are treated this way; a dotted FQDN (db.prod.acme.io)
// or a corporate single-label host not in this list stays a real finding, so a
// genuine leak is never hidden by a coincidental name.
var composeServiceNames = map[string]bool{
	"db": true, "database": true, "postgres": true, "postgresql": true,
	"pg": true, "pgsql": true, "timescaledb": true,
	"mysql": true, "mariadb": true, "mssql": true, "sqlserver": true,
	"mongo": true, "mongodb": true,
	"redis": true, "valkey": true, "cache": true, "memcached": true,
	"rabbitmq": true, "rabbit": true, "amqp": true, "broker": true,
	"kafka": true, "mq": true,
	"elasticsearch": true, "elastic": true, "opensearch": true,
}

// isComposeServiceHost reports a single-label host name that names an
// orchestration service rather than a routable host. An IP is never one.
func isComposeServiceHost(host string) bool {
	h := normalizeHost(host)
	if h == "" || strings.Contains(h, ".") {
		return false
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return false // a bare IPv6 without brackets, etc.
	}
	return composeServiceNames[h]
}

// isDevLocalHost reports a host a host-scoped credential cannot leak to the
// outside world: the local machine, or a compose/k8s service name. Used only
// for database, cache and broker credentials — never for provider API keys,
// which work from anywhere (see LocalOnly).
func isDevLocalHost(host string) bool {
	return IsLocalHost(host) || isComposeServiceHost(host)
}

// IsLocalHost reports whether host refers to the local machine. host may carry
// a port and IPv6 brackets ("[::1]:5432"); case and a trailing dot are ignored.
func IsLocalHost(host string) bool {
	h := normalizeHost(host)
	if h == "" {
		return false
	}

	if localHostNames[h] || strings.HasSuffix(h, ".localhost") { // RFC 6761
		return true
	}

	addr, err := netip.ParseAddr(h)
	if err != nil {
		return false
	}
	addr = addr.Unmap() // ::ffff:127.0.0.1 is still loopback
	return addr.IsLoopback() || addr.IsUnspecified()
}

// LocalURLCredential reports whether secret appears inside a URL in text and
// every URL containing it points only at local hosts —
// postgres://app:secret@localhost/app. This is the safe case to skip
// automatically: the URL itself says which host the secret unlocks.
func LocalURLCredential(text, secret string) bool {
	if secret == "" {
		return false
	}
	found := false
	for _, raw := range urlPattern.FindAllString(text, -1) {
		if !strings.Contains(raw, secret) {
			continue
		}
		found = true
		for _, h := range urlHosts(raw) {
			if !IsLocalHost(h) {
				return false
			}
		}
	}
	return found
}

// hostScopedKeyWords are the words in a setting name that say its value is a
// credential for a specific host — a database, cache or broker — rather than a
// provider API key that works from anywhere. A password under one of these,
// beside a local host, is a local development value.
var hostScopedKeyWords = map[string]bool{
	"db": true, "database": true, "datasource": true, "jdbc": true,
	"postgres": true, "postgresql": true, "pg": true, "pgsql": true,
	"mysql": true, "mariadb": true, "mssql": true, "sqlserver": true, "sql": true,
	"mongo": true, "mongodb": true,
	"redis": true, "cache": true,
	"amqp": true, "rabbit": true, "rabbitmq": true, "broker": true,
}

// hostScopedKey reports whether a setting name identifies a host-scoped service
// credential, so a local host beside it means a local-only value.
func hostScopedKey(key string) bool {
	for _, w := range keyWords(key) {
		if hostScopedKeyWords[w] {
			return true
		}
	}
	return false
}

// neighbourhoodLines is how many lines either side of a match are read for its
// host context. A split DB_HOST/DB_PASSWORD config keeps the two within a few
// lines; a wider window would let an unrelated localhost elsewhere in the file
// hide a real finding, which LocalOnly's own doc warns against.
const neighbourhoodLines = 4

// neighbourhood returns the text of the match's line plus a few lines on each
// side — the narrow window LocalOnly is meant to judge a host-scoped credential
// against.
func neighbourhood(blob []byte, lo, hi int) string {
	start := lo
	for n := 0; n < neighbourhoodLines && start > 0; n++ {
		if i := bytes.LastIndexByte(blob[:start-1], '\n'); i >= 0 {
			start = i + 1
		} else {
			start = 0
		}
	}
	end := hi
	for n := 0; n < neighbourhoodLines && end < len(blob); n++ {
		if i := bytes.IndexByte(blob[end:], '\n'); i >= 0 {
			end = end + i + 1
		} else {
			end = len(blob)
		}
	}
	return string(blob[start:end])
}

// localURL reports whether a connection URL found by a host-scoped rule names
// only local hosts. The secret is the URL itself, so its hosts are the ones
// the credential unlocks; jdbc: is stripped so the scheme parses.
func localURL(secret string) bool {
	hosts := urlHosts(strings.TrimPrefix(secret, "jdbc:"))
	if len(hosts) == 0 {
		return false
	}
	for _, h := range hosts {
		if !isDevLocalHost(h) {
			return false
		}
	}
	return true
}

// allHostsDevLocal reports whether text references at least one host and every
// one is dev-local (loopback, or a compose/k8s service name). It is the
// split-variable counterpart of localURL: the host sits in a DB_HOST= line
// rather than inside the credential URL. Like LocalOnly it is only for
// host-scoped credentials.
func allHostsDevLocal(text string) bool {
	hosts := ReferencedHosts(text)
	if len(hosts) == 0 {
		return false
	}
	for _, h := range hosts {
		if !isDevLocalHost(h) {
			return false
		}
	}
	return true
}

// LocalOnly reports whether every host referenced in text is local, and text
// references at least one host. It returns false when text names no host at
// all, since then nothing shows the credential is confined to one machine.
//
// Only use it for credentials that belong to the host being configured — a
// database, broker or cache password next to DB_HOST=localhost. Never use it
// for provider-issued keys (Stripe, AWS, GitHub tokens): those work from
// anywhere, whatever host the surrounding config points at, so a live key in a
// local .env is still a real leak.
//
// Pass the narrowest text that describes the credential — the matched line
// plus its immediate neighbours — not a whole file. A wide window lets an
// unrelated "localhost" elsewhere in the file hide a real finding.
func LocalOnly(text string) bool {
	hosts := ReferencedHosts(text)
	if len(hosts) == 0 {
		return false
	}
	for _, h := range hosts {
		if !IsLocalHost(h) {
			return false
		}
	}
	return true
}

var (
	// scheme://… anywhere in the text, including inside jdbc:postgresql://…
	urlPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>()\x60]+`)

	// HOST=…, "host": "…", Server=…;, redis_addr: … and similar key/value forms.
	hostKeyPattern = regexp.MustCompile(
		`(?i)[\w.-]*(?:host|hostname|server|addr|address|endpoint)[\w.-]*["']?\s*[:=]\s*["']?([^\s"',;]+)`)
)

// ReferencedHosts extracts host names and addresses from URLs and from
// host-like configuration keys in text. Ports, brackets and userinfo are
// stripped.
func ReferencedHosts(text string) []string {
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		h = normalizeHost(h)
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}

	for _, raw := range urlPattern.FindAllString(text, -1) {
		for _, h := range urlHosts(raw) {
			add(h)
		}
	}

	// URLs are blanked out before the key scan: inside one, "localhost:5432"
	// would otherwise read as a key named localhost with the value 5432.
	rest := urlPattern.ReplaceAllString(text, " ")
	for _, m := range hostKeyPattern.FindAllStringSubmatch(rest, -1) {
		for _, h := range strings.Split(m[1], ",") {
			add(h)
		}
	}

	return hosts
}

// urlHosts returns the hosts in one URL. Multi-host URLs used by MongoDB and
// Postgres (mongodb://u:p@a:27017,b:27017/db) are split, which url.Parse does
// not do on its own.
func urlHosts(raw string) []string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" && !strings.Contains(u.Host, ",") {
		return []string{u.Host}
	}

	// Fallback for what url.Parse rejects or merges: take the authority by hand.
	rest := raw[strings.Index(raw, "://")+3:]
	// ; ends the authority in SQL Server's jdbc:sqlserver://host:1433;user=…
	if i := strings.IndexAny(rest, "/?#;"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	if rest == "" {
		return nil
	}
	return strings.Split(rest, ",")
}

// normalizeHost lowercases and strips port, brackets and a trailing dot.
func normalizeHost(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if h == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	h = strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")
	return strings.TrimSuffix(h, ".")
}
