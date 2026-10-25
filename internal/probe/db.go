package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	mssql "github.com/microsoft/go-mssqldb"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/mservice/gitscanner/internal/scan"
)

// Database probes take a connection URL found in a repository and ask one
// question: does this server accept these credentials. Each opens a
// connection, completes the authentication handshake and closes it — no query
// is run, no database listed, no key read. Getting past authentication is the
// whole signal; "database does not exist" or "no access to this vhost" after a
// successful login still means the password is live.
//
// Unlike the HTTP probes, the target host comes from the repository, not from
// a fixed provider endpoint. That makes the scanner's own network reachable
// from attacker-controlled input, so every connection goes through guardedDial,
// which refuses loopback, link-local (cloud metadata at 169.254.169.254) and
// unspecified addresses after DNS resolution. Private ranges are allowed on
// purpose: an internal database with a leaked password is the finding.

// dbProber is one engine's handshake. dial is the guarded dialer.
type dbProber func(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

var dbProbers = map[string]dbProber{
	"postgres": probePostgres,
	"mysql":    probeMySQL,
	"mongodb":  probeMongo,
	"redis":    probeRedis,
	"rabbitmq": probeRabbit,
	"mssql":    probeMSSQL,
}

// errBlockedAddr is returned by guardedDial for an address the prober must not
// reach. It surfaces as a probe error, never as dead: nothing was learned about
// the credential.
var errBlockedAddr = errors.New("probe: address not allowed")

// guardedDial checks the resolved address, not the host name, so a public name
// that resolves to 127.0.0.1 is refused as well.
func guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{
		Timeout: probeDialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			ip = ip.Unmap()
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
				ip.IsUnspecified() || ip.IsMulticast() {
				return errBlockedAddr
			}
			return nil
		},
	}
	return d.DialContext(ctx, network, addr)
}

// probeDialTimeout bounds the TCP connect. Most leaked database hosts are on a
// private network the scanner cannot route to, and those should fail fast
// rather than consume the engine's whole per-probe budget.
const probeDialTimeout = 4 * time.Second

func errResult(reason string) scan.ProbeResult {
	return scan.ProbeResult{Outcome: scan.ProbeError, Identity: map[string]string{"error": reason}}
}

// netReason names why a connection could not be made, for the operator: an
// unreachable host and a refused address call for different follow-ups.
func netReason(err error) string {
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, errBlockedAddr):
		return "blocked_address"
	case errors.As(err, &dnsErr):
		return "dns"
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	}
	return "unreachable"
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// normalizeDBURL rewrites the credential-carrying forms a rule may capture
// into the userinfo URL the driver parsers accept. A JDBC URL keeps its
// credentials in query parameters (jdbc:postgresql://host/db?user=app&
// password=…) or SQL Server's ;-separated properties, which pgx and the MySQL
// parser do not read; this turns them into scheme://user:pass@host/db. A URL
// already in userinfo form only loses its jdbc: prefix. scheme is the output
// driver scheme for the provider.
func normalizeDBURL(scheme, raw string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimPrefix(raw, "jdbc:"), schemeOf(raw))
	if !ok {
		return "", false
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?;"); i >= 0 {
		authority = rest[:i]
	}
	// Already userinfo form: keep host, path and query, just fix the scheme.
	if strings.Contains(authority, "@") {
		return scheme + "://" + rest, true
	}

	var user, pass, dbName string
	var params []string
	// The tail after the authority holds the path and the key=value settings,
	// separated by ? & or ; depending on the driver's URL dialect.
	tail := rest[len(authority):]
	path := ""
	if strings.HasPrefix(tail, "/") {
		path = tail[1:]
		if i := strings.IndexAny(path, "?;"); i >= 0 {
			path = path[:i]
		}
	}
	for _, kv := range strings.FieldsFunc(tail, func(r rune) bool { return r == '?' || r == '&' || r == ';' }) {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		switch strings.ToLower(k) {
		case "user", "username":
			user = v
		case "password":
			pass = v
		case "databasename", "database":
			dbName = v
		default:
			params = append(params, kv)
		}
	}
	if pass == "" {
		return "", false // nothing to probe without a password
	}
	if dbName == "" {
		dbName = path
	}
	u := scheme + "://" + url.QueryEscape(user) + ":" + url.QueryEscape(pass) + "@" + authority
	if dbName != "" {
		u += "/" + dbName
	}
	if len(params) > 0 {
		u += "?" + strings.Join(params, "&")
	}
	return u, true
}

// schemeOf returns the "scheme://" prefix of raw, after any jdbc: prefix.
func schemeOf(raw string) string {
	raw = strings.TrimPrefix(raw, "jdbc:")
	if i := strings.Index(raw, "://"); i >= 0 {
		return raw[:i+len("://")]
	}
	return ""
}

func probePostgres(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	connURL, ok := normalizeDBURL("postgres", connURL)
	if !ok {
		return errResult("bad_url")
	}
	cfg, err := pgx.ParseConfig(connURL)
	if err != nil {
		return errResult("bad_url")
	}
	cfg.DialFunc = pgconn.DialFunc(dial)
	cfg.ConnectTimeout = probeDialTimeout
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "28P01", "28000": // invalid_password, invalid_authorization_specification
				return scan.ProbeResult{Outcome: scan.ProbeDead}
			case "3D000": // invalid_catalog_name: logged in, the database is just missing
				return scan.ProbeResult{Outcome: scan.ProbeLive}
			}
			return errResult("server_" + pgErr.Code)
		}
		return errResult(netReason(err))
	}
	defer func() { _ = conn.Close(context.Background()) }()
	out := scan.ProbeResult{Outcome: scan.ProbeLive}
	if v := conn.PgConn().ParameterStatus("server_version"); v != "" {
		out.Identity = map[string]string{"server_version": v}
	}
	return out
}

func probeMySQL(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	connURL, ok := normalizeDBURL("mysql", connURL)
	if !ok {
		return errResult("bad_url")
	}
	u, err := url.Parse(connURL)
	if err != nil || u.User == nil {
		return errResult("bad_url")
	}
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	if u.Port() == "" {
		cfg.Addr = net.JoinHostPort(u.Hostname(), "3306")
	}
	cfg.User = u.User.Username()
	cfg.Passwd, _ = u.User.Password()
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.TLSConfig = "preferred"
	cfg.Timeout = probeDialTimeout
	cfg.DialFunc = dial
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return errResult("bad_url")
	}
	conn, err := connector.Connect(ctx)
	if err != nil {
		var myErr *mysql.MySQLError
		if errors.As(err, &myErr) {
			switch myErr.Number {
			case 1045: // ER_ACCESS_DENIED_ERROR: wrong user or password
				return scan.ProbeResult{Outcome: scan.ProbeDead}
			case 1044, 1049: // no access to / unknown database: authenticated first
				return scan.ProbeResult{Outcome: scan.ProbeLive}
			}
			return errResult(fmt.Sprintf("server_%d", myErr.Number))
		}
		return errResult(netReason(err))
	}
	_ = conn.Close()
	return scan.ProbeResult{Outcome: scan.ProbeLive}
}

type contextDialer dialFunc

func (d contextDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d(ctx, network, addr)
}

func probeMongo(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	opts := options.Client().ApplyURI(connURL).
		SetDialer(contextDialer(dial)).
		SetConnectTimeout(probeDialTimeout).
		SetServerSelectionTimeout(probeDialTimeout).
		SetMaxPoolSize(1)
	if opts.Validate() != nil {
		return errResult("bad_url")
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return errResult("bad_url")
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	err = client.Ping(ctx, readpref.Nearest())
	if err == nil {
		return scan.ProbeResult{Outcome: scan.ProbeLive}
	}
	// The driver authenticates during the connection handshake, so a bad
	// password comes back wrapped in a server-selection error rather than as a
	// typed CommandError; the server's code name is the stable part of it.
	msg := err.Error()
	if strings.Contains(msg, "AuthenticationFailed") || strings.Contains(msg, "auth error") {
		return scan.ProbeResult{Outcome: scan.ProbeDead}
	}
	return errResult(netReason(err))
}

func probeRedis(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	opts, err := redis.ParseURL(connURL)
	if err != nil {
		return errResult("bad_url")
	}
	opts.Dialer = dial
	opts.DialTimeout = probeDialTimeout
	opts.MaxRetries = -1 // one attempt: a retry is a second login with someone's password
	opts.PoolSize = 1
	opts.DisableIdentity = true
	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()
	err = client.Ping(ctx).Err()
	if err == nil {
		return scan.ProbeResult{Outcome: scan.ProbeLive}
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "WRONGPASS") || strings.Contains(msg, "invalid password") ||
		strings.Contains(msg, "invalid username-password") {
		return scan.ProbeResult{Outcome: scan.ProbeDead}
	}
	if strings.HasPrefix(msg, "NOAUTH") || strings.HasPrefix(msg, "ERR") {
		return errResult("server_rejected")
	}
	return errResult(netReason(err))
}

func probeMSSQL(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	norm, ok := normalizeDBURL("sqlserver", connURL)
	if !ok {
		return errResult("bad_url")
	}
	u, err := url.Parse(norm)
	if err != nil || u.User == nil {
		return errResult("bad_url")
	}
	// SQL Server's URL form names the database as a query parameter, not a path
	// segment (a path there is an instance name); move it across.
	q := u.Query()
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		q.Set("database", db)
	}
	u.Path = ""
	q.Set("dial timeout", "4")
	q.Set("connection timeout", "4")
	// A probe only needs to authenticate; disable encryption so a self-signed
	// server certificate does not turn a live credential into a handshake error.
	q.Set("encrypt", "disable")
	q.Set("TrustServerCertificate", "true")
	u.RawQuery = q.Encode()

	connector, err := mssql.NewConnector(u.String())
	if err != nil {
		return errResult("bad_url")
	}
	connector.Dialer = contextDialer(dial)
	conn, err := connector.Connect(ctx)
	if err != nil {
		var me mssql.Error
		if errors.As(err, &me) {
			switch me.Number {
			case 18456: // login failed for user: wrong credentials
				return scan.ProbeResult{Outcome: scan.ProbeDead}
			case 4060, 4063, 911: // cannot open the requested database: authenticated first
				return scan.ProbeResult{Outcome: scan.ProbeLive}
			}
			return errResult(fmt.Sprintf("server_%d", me.Number))
		}
		return errResult(netReason(err))
	}
	_ = conn.Close()
	return scan.ProbeResult{Outcome: scan.ProbeLive}
}

func probeRabbit(ctx context.Context, dial dialFunc, connURL string) scan.ProbeResult {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(probeDialTimeout)
	}
	cfg := amqp.Config{
		// amqp's Dial takes no context; the deadline on the socket is what
		// bounds the handshake, as amqp.DefaultDial does for its own.
		Dial: func(network, addr string) (net.Conn, error) {
			c, err := dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			_ = c.SetDeadline(deadline)
			return c, nil
		},
	}
	conn, err := amqp.DialConfig(connURL, cfg)
	if err != nil {
		var aErr *amqp.Error
		switch {
		case errors.As(err, &aErr) && aErr.Reason == amqp.ErrVhost.Reason:
			return scan.ProbeResult{Outcome: scan.ProbeLive} // logged in, vhost denied
		case errors.As(err, &aErr) && aErr.Code == amqp.AccessRefused:
			return scan.ProbeResult{Outcome: scan.ProbeDead}
		case errors.As(err, &aErr):
			return errResult(fmt.Sprintf("server_%d", aErr.Code))
		}
		return errResult(netReason(err))
	}
	_ = conn.Close()
	return scan.ProbeResult{Outcome: scan.ProbeLive}
}
