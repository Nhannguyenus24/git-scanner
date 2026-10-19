package scan_test

import (
	"reflect"
	"testing"

	"github.com/mservice/gitscanner/internal/scan"
)

func TestIsLocalHost(t *testing.T) {
	local := []string{
		"localhost", "LOCALHOST", "localhost.", "localhost:5432",
		"api.localhost", // RFC 6761: all of .localhost is loopback
		"127.0.0.1", "127.8.9.10", "127.0.0.1:6379",
		"::1", "[::1]", "[::1]:5432", "::ffff:127.0.0.1",
		"0.0.0.0", "0.0.0.0:8080", "::",
		"host.docker.internal", "host.k3d.internal", "host.containers.internal",
	}
	for _, h := range local {
		if !scan.IsLocalHost(h) {
			t.Errorf("IsLocalHost(%q) = false, want true", h)
		}
	}

	// Private ranges and internal names are shared infrastructure: a leaked
	// credential for them is a real finding and must still be processed.
	notLocal := []string{
		"", "db.example.com", "localhost.example.com", "mylocalhost",
		"10.0.0.5", "172.16.0.1", "192.168.1.10", "169.254.169.254",
		"pg.infra.internal", "8.8.8.8", "2001:db8::1",
	}
	for _, h := range notLocal {
		if scan.IsLocalHost(h) {
			t.Errorf("IsLocalHost(%q) = true, want false", h)
		}
	}
}

func TestLocalOnly(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// Skipped: everything points at the developer's machine.
		{"postgres url", "DATABASE_URL=postgresql://postgres:postgres123@localhost:5432/app_db", true},
		{"amqp url", "RABBITMQ_URL=amqp://admin:admin123@127.0.0.1:5672/", true},
		{"redis url ipv6", `REDIS_URL="redis://:redis123@[::1]:6379/0"`, true},
		{"jdbc", "spring.datasource.url=jdbc:postgresql://localhost:5432/app", true},
		{"mongo multi-host", "mongodb://u:p@localhost:27017,127.0.0.1:27018/db?replicaSet=rs0", true},
		{"env host key", "DB_HOST=127.0.0.1\nDB_PASSWORD=s3cr3t-value", true},
		{"yaml host key", "database:\n  host: localhost\n  password: hunter2hunter2", true},
		{"json host key", `{"host": "localhost", "password": "hunter2hunter2"}`, true},
		{"ado connection string", "Server=localhost;Database=app;User Id=sa;Password=Str0ngP@ss;", true},
		{"docker host alias", "SMTP_HOST=host.docker.internal", true},

		// Processed: at least one host is reachable from elsewhere.
		{"remote url", "postgresql://app:pw@db.prod.example.com:5432/app", false},
		{"private ip", "DB_HOST=10.0.3.17\nDB_PASSWORD=s3cr3t-value", false},
		{"mixed hosts", "mongodb://u:p@localhost:27017,db1.example.com:27017/db", false},
		{"local and remote keys", "PRIMARY_HOST=localhost\nREPLICA_HOST=replica.example.com", false},

		// Processed: no host at all, so nothing shows the key is local-only.
		{"bare key", "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", false},
		{"empty", "", false},
	}

	for _, c := range cases {
		if got := scan.LocalOnly(c.text); got != c.want {
			t.Errorf("%s: LocalOnly = %v, want %v (hosts %q)",
				c.name, got, c.want, scan.ReferencedHosts(c.text))
		}
	}
}

func TestReferencedHostsStripsPortsAndDedupes(t *testing.T) {
	got := scan.ReferencedHosts("A=redis://localhost:6379/0 B_HOST=localhost:6379 C=https://api.example.com/v1")
	want := []string{"localhost", "api.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReferencedHosts = %q, want %q", got, want)
	}
}

func TestLocalURLCredential(t *testing.T) {
	dummyKey := "sk_live" + "_51H8xKpLmNqRsTuVwXyZ"
	env := "DATABASE_URL=postgres://app:pg-local-pw@localhost:5432/app\n" +
		"STRIPE_SECRET_KEY=" + dummyKey + "\n" +
		"BACKUP_URL=postgres://app:pg-prod-pw@db.example.com/app\n"

	cases := []struct {
		secret string
		want   bool
	}{
		{"pg-local-pw", true}, // only ever used against localhost
		{dummyKey, false},     // not in a URL: a provider key, still a leak
		{"pg-prod-pw", false}, // in a URL, but a remote one
		{"", false},
	}
	for _, c := range cases {
		if got := scan.LocalURLCredential(env, c.secret); got != c.want {
			t.Errorf("LocalURLCredential(%q) = %v, want %v", c.secret, got, c.want)
		}
	}

	// The same secret reused against a remote host must not be skipped.
	reused := "A=redis://:shared-pw@localhost:6379 B=redis://:shared-pw@cache.example.com:6379"
	if scan.LocalURLCredential(reused, "shared-pw") {
		t.Error("a secret also used against a remote host was treated as local-only")
	}
}
