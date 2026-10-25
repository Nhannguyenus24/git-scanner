package probe_test

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/probe"
	"github.com/mservice/gitscanner/internal/scan"
)

// The real dialer refuses loopback, so a repository cannot point the prober at
// a service on the scanner's own host.
func TestDBProbeRefusesLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	got := probe.New(nil).Probe(context.Background(), "redis", "redis://:pw@"+ln.Addr().String())
	if got.Outcome != scan.ProbeError || got.Identity["error"] != "blocked_address" {
		t.Errorf("got %+v, want error/blocked_address", got)
	}
}

// fakeRedis answers every command with reply, which is enough to drive the
// client's AUTH/PING handshake to the outcome under test.
func fakeRedis(t *testing.T, reply func(cmd string) string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				r := bufio.NewReader(c)
				for {
					cmd, err := readCommand(r)
					if err != nil {
						return
					}
					_, _ = c.Write([]byte(reply(cmd)))
				}
			}()
		}
	}()
	return ln.Addr().String()
}

// readCommand reads one RESP array and returns its first element, upper-cased.
func readCommand(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	n := 0
	for _, c := range strings.TrimSpace(line[1:]) {
		n = n*10 + int(c-'0')
	}
	var first string
	for i := 0; i < n; i++ {
		if _, err := r.ReadString('\n'); err != nil { // $len
			return "", err
		}
		arg, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		if i == 0 {
			first = strings.ToUpper(strings.TrimSpace(arg))
		}
	}
	return first, nil
}

func loopbackProber() *probe.Prober {
	var d net.Dialer
	return probe.New(nil).WithDialer(d.DialContext)
}

func TestDBProbeRedisLive(t *testing.T) {
	addr := fakeRedis(t, func(cmd string) string {
		switch cmd {
		case "HELLO":
			return "-ERR unknown command 'HELLO'\r\n"
		case "PING":
			return "+PONG\r\n"
		}
		return "+OK\r\n"
	})
	got := loopbackProber().Probe(context.Background(), "redis", "redis://:pw@"+addr)
	if got.Outcome != scan.ProbeLive {
		t.Errorf("got %+v, want live", got)
	}
}

func TestDBProbeRedisWrongPassword(t *testing.T) {
	addr := fakeRedis(t, func(cmd string) string {
		if cmd == "HELLO" {
			return "-ERR unknown command 'HELLO'\r\n"
		}
		return "-WRONGPASS invalid username-password pair or user is disabled.\r\n"
	})
	got := loopbackProber().Probe(context.Background(), "redis", "redis://:pw@"+addr)
	if got.Outcome != scan.ProbeDead {
		t.Errorf("got %+v, want dead", got)
	}
}

// A host the scanner cannot resolve is an error, not a dead credential.
func TestDBProbeUnresolvableHost(t *testing.T) {
	got := probe.New(nil).Probe(context.Background(), "postgres", "postgres://u:pw@nonexistent.invalid:5432/db")
	if got.Outcome != scan.ProbeError {
		t.Errorf("got %+v, want error", got)
	}
}

// An unresolvable SQL Server host is an error, not a dead credential, and the
// guarded dialer refuses a loopback target.
func TestDBProbeMSSQL(t *testing.T) {
	if got := probe.New(nil).Probe(context.Background(), "mssql",
		"sqlserver://sa:pw@nonexistent.invalid:1433?database=erp"); got.Outcome != scan.ProbeError {
		t.Errorf("unresolvable: got %+v, want error", got)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if got := probe.New(nil).Probe(context.Background(), "mssql",
		"jdbc:sqlserver://"+ln.Addr().String()+";user=sa;password=pw"); got.Identity["error"] != "blocked_address" {
		t.Errorf("loopback: got %+v, want blocked_address", got)
	}
}
