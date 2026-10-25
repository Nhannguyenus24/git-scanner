package scan

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// A database credential is often not a single connection URL but a set of
// separate settings — DB_HOST, DB_PORT, DB_USER, DB_PASSWORD — which the URL
// rules in bootstrap_rules.go cannot see. assembleDBConn reconstructs the
// connection those settings describe, so a split credential can be attributed
// to its engine and validated the same way a URL one is.
//
// It is deliberately conservative: it returns ok only when the engine is
// unambiguous and the host is routable. Guessing the engine would make the
// prober speak the wrong protocol to a real host; guessing the host would point
// it somewhere the credential does not belong. When either is uncertain the
// finding stays an unattributed generic one, caught but not probed.

// dbConn is a reconstructed database connection. Only non-secret fields but the
// password, which is the already-detected value; the whole struct lives and
// dies inside detection and is never stored.
type dbConn struct {
	engine, host, port, user, db, password string
}

// defaultPorts lets a host with a known engine but no DB_PORT still be probed.
var defaultPorts = map[string]string{
	"postgres": "5432", "mysql": "3306", "mongodb": "27017",
	"redis": "6379", "rabbitmq": "5672", "mssql": "1433",
}

// portEngines maps a port back to the engine it conventionally serves, the
// fallback when the setting names do not say which engine is meant (a plain
// DB_HOST/DB_PORT/DB_PASSWORD trio).
var portEngines = map[string]string{
	"5432": "postgres", "5433": "postgres",
	"3306":  "mysql",
	"27017": "mongodb", "27018": "mongodb",
	"6379": "redis", "6380": "redis",
	"5672": "rabbitmq", "5671": "rabbitmq",
	"1433": "mssql",
}

// engineKeyWords maps a word in a setting name to the engine it names, so
// POSTGRES_PASSWORD, MYSQL_HOST or REDIS_URL fix the engine regardless of port.
var engineKeyWords = map[string]string{
	"postgres": "postgres", "postgresql": "postgres", "pg": "postgres", "pgsql": "postgres",
	"mysql": "mysql", "mariadb": "mysql",
	"mongo": "mongodb", "mongodb": "mongodb",
	"redis":    "redis",
	"rabbitmq": "rabbitmq", "rabbit": "rabbitmq", "amqp": "rabbitmq",
	"mssql": "mssql", "sqlserver": "mssql",
}

// defaultUsers is the conventional superuser when no DB_USER is set, so a probe
// can still authenticate. Redis and MongoDB authenticate with no user by
// default, so they have none.
var defaultUsers = map[string]string{
	"postgres": "postgres", "mysql": "root", "mssql": "sa",
}

// envAssignPattern captures the name and value of every KEY=value / KEY: value /
// "KEY": "value" assignment on a line. Compiled once; the neighbourhood is
// parsed into a map rather than re-scanned per setting.
var envAssignPattern = regexp.MustCompile(`(?m)([A-Za-z][A-Za-z0-9_.-]*)["']?\s*[:=]\s*["']?([^\s"',;]+)`)

// parseAssignments returns the key→value pairs in text, keys lowercased. A
// repeated key keeps its last value, matching how env files override.
func parseAssignments(text string) map[string]string {
	out := map[string]string{}
	for _, m := range envAssignPattern.FindAllStringSubmatch(text, -1) {
		out[strings.ToLower(m[1])] = m[2]
	}
	return out
}

// bySuffix returns the value of the assignment whose key's last word is one of
// suffixes (host, port, user, …), so DB_HOST and POSTGRES_HOST both answer
// "host" without matching a mid-name coincidence.
func bySuffix(kv map[string]string, suffixes ...string) string {
	for k, v := range kv {
		words := keyWords(k)
		if len(words) == 0 {
			continue
		}
		last := words[len(words)-1]
		for _, s := range suffixes {
			if last == s {
				return v
			}
		}
	}
	return ""
}

// assembleDBConn reconstructs a connection from the settings around a detected
// password. key is the setting the password was assigned to, neigh the few
// lines around it, password the detected value. ok is false unless the engine
// is unambiguous and the host is routable.
func assembleDBConn(key, neigh, password string) (dbConn, bool) {
	c := dbConn{password: password}

	// Host first: without a routable one there is nothing to probe.
	c.host = firstRoutableHost(neigh)
	if c.host == "" {
		return dbConn{}, false
	}

	kv := parseAssignments(neigh)
	c.port = bySuffix(kv, "port")
	c.user = bySuffix(kv, "user", "username")
	c.db = bySuffix(kv, "database", "dbname", "name")

	// Engine: a name in the keys wins; otherwise the port decides. No guess
	// beyond that.
	c.engine = engineFromKeys(key, kv)
	if c.engine == "" {
		c.engine = portEngines[c.port]
	}
	if c.engine == "" {
		return dbConn{}, false
	}
	if c.port == "" {
		c.port = defaultPorts[c.engine]
	}
	if c.user == "" {
		c.user = defaultUsers[c.engine]
	}
	return c, true
}

// engineFromKeys returns the engine named by any setting key, or "". It reads
// every key, not just the password's, so a DB_PASSWORD beside a POSTGRES_HOST is
// still identified.
func engineFromKeys(key string, kv map[string]string) string {
	for _, w := range keyWords(key) {
		if e := engineKeyWords[w]; e != "" {
			return e
		}
	}
	for k := range kv {
		for _, w := range keyWords(k) {
			if e := engineKeyWords[w]; e != "" {
				return e
			}
		}
	}
	return ""
}

// firstRoutableHost returns the first host in text that is neither dev-local nor
// a compose service name — the one worth probing.
func firstRoutableHost(text string) string {
	for _, h := range ReferencedHosts(text) {
		if !isDevLocalHost(h) {
			return h
		}
	}
	return ""
}

// url builds the connection URL the prober validates. For MongoDB the user may
// be empty (it authenticates against a database); for Redis the user is
// customarily empty and the password sits alone in the userinfo.
func (c dbConn) url() string {
	scheme := c.engine
	if scheme == "mssql" {
		scheme = "sqlserver"
	}
	host := c.host
	if c.port != "" {
		host = net.JoinHostPort(c.host, c.port)
	}
	userinfo := ""
	switch {
	case c.user != "":
		userinfo = url.QueryEscape(c.user) + ":" + url.QueryEscape(c.password) + "@"
	default:
		userinfo = ":" + url.QueryEscape(c.password) + "@"
	}
	u := scheme + "://" + userinfo + host
	if c.db != "" {
		u += "/" + c.db
	}
	return u
}
