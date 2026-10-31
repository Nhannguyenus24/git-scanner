package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mservice/gitscanner/internal/assist"
)

type fakeTranslator struct {
	question  string
	providers []string
	err       error
}

func (f *fakeTranslator) Translate(_ context.Context, q string, providers []string) (assist.Filter, error) {
	f.question, f.providers = q, providers
	if f.err != nil {
		return assist.Filter{}, f.err
	}
	return assist.Filter{Severity: "critical", Explanation: "critical ones"}, nil
}

func TestAssistRoutes(t *testing.T) {
	srv, _, _ := newServer(t)
	if rec := do(srv, http.MethodGet, "/v1/assist", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("assist status without a translator: %d, want 404", rec.Code)
	}

	tr := &fakeTranslator{}
	srv.WithAssist(tr, "claude-opus-5-5")
	rec := do(srv, http.MethodPost, "/v1/assist/filter", `{"question":"critical keys"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var f assist.Filter
	_ = json.Unmarshal(rec.Body.Bytes(), &f)
	if f.Severity != "critical" || tr.question != "critical keys" {
		t.Fatalf("filter %+v, question %q", f, tr.question)
	}

	for err, want := range map[error]int{
		assist.ErrCredentialLike: http.StatusBadRequest,
		assist.ErrRefused:        http.StatusUnprocessableEntity,
		context.DeadlineExceeded: http.StatusBadGateway,
	} {
		tr.err = err
		if rec := do(srv, http.MethodPost, "/v1/assist/filter", `{"question":"x"}`); rec.Code != want {
			t.Errorf("%v: status %d, want %d", err, rec.Code, want)
		}
	}
}
