package assist_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/mservice/gitscanner/internal/assist"
)

func TestCheckQuestionRefusesCredentialShapes(t *testing.T) {
	for _, q := range []string{
		"why is AKIAZ7Q2EXAMPLEKEY4ABCD in my repo",
		"find ghp_1234567890abcdefghijABCDEFGHIJ",
		"postgres://admin:hunter2@db.internal/app",
	} {
		if _, err := assist.CheckQuestion(q); !errors.Is(err, assist.ErrCredentialLike) {
			t.Errorf("%q: err = %v, want ErrCredentialLike", q, err)
		}
	}
	for _, q := range []string{
		"critical AWS keys still at HEAD in org acme",
		"khóa stripe còn sống trong kho payments-service-backend",
		"oldest open findings in config/application-production.yml",
	} {
		if _, err := assist.CheckQuestion(q); err != nil {
			t.Errorf("%q: unexpected %v", q, err)
		}
	}
	if _, err := assist.CheckQuestion("   "); !errors.Is(err, assist.ErrEmpty) {
		t.Errorf("blank: %v", err)
	}
	if _, err := assist.CheckQuestion(strings.Repeat("a ", assist.MaxQuestion)); !errors.Is(err, assist.ErrTooLong) {
		t.Errorf("long: %v", err)
	}
}

func TestSanitizeDropsWhatTheDashboardCannotTake(t *testing.T) {
	got := assist.Sanitize(assist.Filter{
		Severity: "CRITICAL", Provider: "made-up", Status: "open; DROP", Exposure: "head",
		Sort: "random", Q: "ghp_1234567890abcdefghijABCDEFGHIJ", Explanation: " critical keys at HEAD ",
	}, []string{"aws", "stripe"})
	want := assist.Filter{Severity: "critical", Exposure: "head", Explanation: "critical keys at HEAD"}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if p := assist.Sanitize(assist.Filter{Provider: "unmapped"}, nil).Provider; p != "unmapped" {
		t.Errorf("unmapped dropped: %q", p)
	}
}

// The request is checked on the wire: the right model and settings, and no
// content beyond the question and the provider list.
func TestClaudeRequestAndAnswer(t *testing.T) {
	var body map[string]any
	var beta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		beta = r.Header.Get("anthropic-beta")
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
			"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1},
			"content":[{"type":"text","text":"{\"severity\":\"critical\",\"provider\":\"aws\",\"status\":\"open\",\"exposure\":\"head\",\"sort\":\"\",\"q\":\"acme\",\"explanation\":\"Critical AWS keys at HEAD in acme\"}"}]}`)
	}))
	defer srv.Close()

	c := assist.NewClaude("test-key", option.WithBaseURL(srv.URL))
	f, err := c.Translate(context.Background(), "critical AWS keys still at HEAD in org acme", []string{"aws", "stripe"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Severity != "critical" || f.Provider != "aws" || f.Exposure != "head" || f.Q != "acme" {
		t.Fatalf("filter %+v", f)
	}
	if body["model"] != assist.Model {
		t.Errorf("model %v", body["model"])
	}
	if body["fallbacks"] != "default" || !strings.Contains(beta, "server-side-fallback-2026-07-01") {
		t.Errorf("fallbacks %v, beta %q", body["fallbacks"], beta)
	}
	oc, _ := body["output_config"].(map[string]any)
	if oc["effort"] != "low" || oc["format"] == nil {
		t.Errorf("output_config %v", oc)
	}
	msgs, _ := json.Marshal(body["messages"])
	if !strings.Contains(string(msgs), "Question: critical AWS keys") || strings.Count(string(msgs), `"role"`) != 1 {
		t.Errorf("messages carry more than the question: %s", msgs)
	}
}

func TestClaudeRefusalIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"refusal","usage":{"input_tokens":1,"output_tokens":0},"content":[]}`)
	}))
	defer srv.Close()
	_, err := assist.NewClaude("k", option.WithBaseURL(srv.URL)).Translate(context.Background(), "open findings", nil)
	if !errors.Is(err, assist.ErrRefused) {
		t.Fatalf("err = %v, want ErrRefused", err)
	}
}

func TestClaudeNeverCallsOutForACredentialLikeQuestion(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	_, err := assist.NewClaude("k", option.WithBaseURL(srv.URL)).Translate(context.Background(), "is sk_live_51Habcdefghijklmnop1234 live?", nil)
	if !errors.Is(err, assist.ErrCredentialLike) || called {
		t.Fatalf("err = %v, called = %v", err, called)
	}
}
