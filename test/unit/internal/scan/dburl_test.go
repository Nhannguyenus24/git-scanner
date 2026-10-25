package scan_test

import (
	"strings"
	"testing"
)

// Each engine's connection URL is attributed to its own provider, and the
// secret is the whole URL so the prober has the host as well as the password.
func TestDetectDatabaseURLs(t *testing.T) {
	cases := map[string]string{
		"postgres": "DATABASE_URL=postgresql://app:Qz7vLm2pRt9x@db.prod.acme.io:5432/orders",
		"mysql":    `spring.datasource.url=jdbc:mysql://svc:Qz7vLm2pRt9x@10.20.0.15:3306/shop`,
		"mongodb":  `MONGO_URI="mongodb+srv://svc:Qz7vLm2pRt9x@cluster0.ab1cd.mongodb.net/app?retryWrites=true"`,
		"redis":    "REDIS_URL=rediss://:Qz7vLm2pRt9x@cache.acme.io:6380/0",
		"rabbitmq": "AMQP_URL=amqps://svc:Qz7vLm2pRt9x@mq.acme.io/vhost",
		"mssql":    "conn: sqlserver://sa:Qz7vLm2pRt9x@sql.acme.io:1433?database=erp",
	}
	for provider, line := range cases {
		ms := detect("config.env", line)
		if len(ms) != 1 || ms[0].Provider != provider {
			t.Errorf("%s: got %+v, want one %s finding", provider, ms, provider)
			continue
		}
		if ms[0].Secret[len(ms[0].Secret)-1] == '"' {
			t.Errorf("%s: secret swallowed the closing quote: %q", provider, ms[0].Secret)
		}
	}
}

// A URL that points only at the developer's machine is a local value, and the
// generic pass must not pick its password back out either.
func TestDatabaseURLLocalhostDropped(t *testing.T) {
	for _, line := range []string{
		"DATABASE_URL=postgres://app:Qz7vLm2pRt9xWk4sHn8@localhost:5432/app",
		"REDIS_URL=redis://:Qz7vLm2pRt9xWk4sHn8@127.0.0.1:6379",
		"MONGO=mongodb://u:Qz7vLm2pRt9xWk4sHn8@host.docker.internal:27017/db",
		"mysql://root:Qz7vLm2pRt9xWk4sHn8@[::1]:3306/x",
	} {
		if ms := detect("config.env", line); len(ms) != 0 {
			t.Errorf("local URL reported: %q -> %+v", line, ms)
		}
	}
}

// A multi-host URL is local only if every host is.
func TestDatabaseURLMixedHostsKept(t *testing.T) {
	ms := detect("config.env", "mongodb://u:Qz7vLm2pRt9x@localhost:27017,db2.acme.io:27017/app")
	if len(ms) != 1 {
		t.Errorf("mixed local/remote hosts: %d matches, want 1", len(ms))
	}
}

// The local dev URL above the production one used to hide it: a rule yielded
// only its first match per blob.
func TestDatabaseURLAfterLocalOneIsFound(t *testing.T) {
	blob := "dev: postgres://app:devpw4321zz@localhost/app\n" +
		"prod: postgres://app:Qz7vLm2pRt9x@db.prod.acme.io/app\n"
	ms := detect("config.yml", blob)
	if len(ms) != 1 || ms[0].Line != 2 {
		t.Errorf("got %+v, want the production URL on line 2", ms)
	}
}

func TestDatabaseURLPlaceholdersAndNoUserinfo(t *testing.T) {
	for _, line := range []string{
		"postgres://user:password@db.acme.io/app",
		"postgres://app:${DB_PASSWORD}@db.acme.io/app",
		"postgres://app:<password>@db.acme.io/app",
		"postgres://db.acme.io:5432/app?sslmode=require&x=a@b",
	} {
		if ms := detect("README.md", line); len(ms) != 0 {
			t.Errorf("reported: %q -> %+v", line, ms)
		}
	}
}

// JDBC drivers carry the credentials as query parameters or ;-properties, not
// in userinfo; those must be attributed and captured whole too.
func TestDetectJDBCURLs(t *testing.T) {
	cases := map[string]string{
		"postgres": "db.url=jdbc:postgresql://db.prod.acme.io:5432/orders?user=app&password=Qz7vLm2pRt9x&ssl=true",
		"mysql":    "spring.datasource.url=jdbc:mysql://10.20.0.15:3306/shop?user=svc&password=Qz7vLm2pRt9x",
		"mssql":    "jdbc:sqlserver://sql.acme.io:1433;databaseName=erp;user=sa;password=Qz7vLm2pRt9x;encrypt=true",
	}
	for provider, line := range cases {
		ms := detect("application.properties", line)
		if len(ms) != 1 || ms[0].Provider != provider {
			t.Errorf("%s: got %+v, want one %s finding", provider, ms, provider)
		}
	}
}

// A localhost JDBC URL is a local value; a ${...} password is a placeholder.
func TestJDBCURLSuppressed(t *testing.T) {
	for _, line := range []string{
		"jdbc:postgresql://localhost:5432/app?user=app&password=Qz7vLm2pRt9xWk",
		"jdbc:mysql://127.0.0.1:3306/app?user=root&password=${DB_PASSWORD}",
	} {
		if ms := detect("application.properties", line); len(ms) != 0 {
			t.Errorf("suppressed JDBC URL reported: %q -> %+v", line, ms)
		}
	}
}

// A password split from its host across separate variables is a local value
// when every host named nearby is local — the case the URL rules cannot see.
func TestSplitDBVarsLocalhostDropped(t *testing.T) {
	env := "DB_HOST=localhost\nDB_PORT=5432\nDB_USER=app\nDB_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n"
	if ms := detect(".env", env); len(ms) != 0 {
		t.Errorf("local split DB vars reported: %+v", ms)
	}
}

// But a split password for a remote host is still a finding, and a provider key
// near a stray localhost line is not hidden by the host-scoped suppression.
func TestSplitDBVarsRemoteKept(t *testing.T) {
	env := "DB_HOST=db.prod.acme.io\nDB_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n"
	if ms := detect(".env", env); len(ms) != 1 {
		t.Errorf("remote split DB password: %d matches, want 1", len(ms))
	}
	// A non-DB secret keeps being reported even beside DB_HOST=localhost: the
	// suppression is scoped to host-scoped keys only.
	mixed := "DB_HOST=localhost\nAPP_SECRET=Qz7vLm2pRt9xWk4sHn8aB3\n"
	if ms := detect(".env", mixed); len(ms) != 1 {
		t.Errorf("non-DB secret near localhost: %d matches, want 1", len(ms))
	}
}

// A single-label compose/k8s service name is a dev-network reference, not a
// routable leak: dropped like localhost, for URLs and split variables alike.
func TestComposeServiceHostDropped(t *testing.T) {
	for _, line := range []string{
		"DATABASE_URL=postgres://app:Qz7vLm2pRt9xWk4@db:5432/app",
		"REDIS_URL=redis://:Qz7vLm2pRt9xWk4@redis:6379",
		"AMQP_URL=amqp://svc:Qz7vLm2pRt9xWk4@rabbitmq:5672/",
	} {
		if ms := detect("docker-compose.yml", line); len(ms) != 0 {
			t.Errorf("compose-service URL reported: %q -> %+v", line, ms)
		}
	}
	env := "POSTGRES_HOST=db\nPOSTGRES_USER=app\nPOSTGRES_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n"
	if ms := detect(".env", env); len(ms) != 0 {
		t.Errorf("compose-service split vars reported: %+v", ms)
	}
	// But a dotted FQDN with the same leading label is still a real finding.
	if ms := detect(".env", "DB_URL=postgres://app:Qz7vLm2pRt9xWk4@db.prod.acme.io:5432/app"); len(ms) != 1 {
		t.Errorf("FQDN starting with db: %d matches, want 1", len(ms))
	}
}

// A remote split credential whose engine is named by a key, or implied by its
// port, is attributed to that engine and carries a probe URL built from the
// siblings. The stored secret stays the password.
func TestSplitVarsAttributedAndProbeable(t *testing.T) {
	cases := []struct {
		name, env, engine, wantInURL string
	}{
		{"by-key", "POSTGRES_HOST=db.prod.acme.io\nPOSTGRES_USER=app\nPOSTGRES_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n", "postgres", "db.prod.acme.io:5432"},
		{"by-port", "DB_HOST=cache.acme.io\nDB_PORT=6379\nDB_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n", "redis", "cache.acme.io:6379"},
		{"mysql-def-user", "MYSQL_HOST=sql.acme.io\nMYSQL_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n", "mysql", "root:"},
	}
	for _, c := range cases {
		ms := detect(".env", c.env)
		if len(ms) != 1 {
			t.Errorf("%s: %d matches, want 1", c.name, len(ms))
			continue
		}
		if ms[0].Provider != c.engine || ms[0].RuleID != "db-split-env" {
			t.Errorf("%s: provider=%s rule=%s, want %s/db-split-env", c.name, ms[0].Provider, ms[0].RuleID, c.engine)
		}
		if ms[0].ProbeURL == "" || !strings.Contains(ms[0].ProbeURL, c.wantInURL) {
			t.Errorf("%s: ProbeURL=%q, want to contain %q", c.name, ms[0].ProbeURL, c.wantInURL)
		}
		if strings.Contains(ms[0].Secret, "://") {
			t.Errorf("%s: stored secret should be the password, not the URL: %q", c.name, ms[0].Secret)
		}
	}
}

// When the engine cannot be determined, the finding stays generic (still caught)
// rather than being attributed to a guessed engine and probed wrongly.
func TestSplitVarsUnknownEngineStaysGeneric(t *testing.T) {
	env := "DB_HOST=store.acme.io\nDB_PASSWORD=Qz7vLm2pRt9xWk4sHn8aB3\n" // no engine word, no known port
	ms := detect(".env", env)
	if len(ms) != 1 || ms[0].Provider != "" || ms[0].RuleID != "generic-high-entropy" {
		t.Errorf("got %+v, want one generic unattributed finding", ms)
	}
}
