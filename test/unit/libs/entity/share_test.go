package entity_test

import (
	"testing"
	"time"

	"github.com/mservice/gitscanner/libs/entity"
)

func TestShareLinkState(t *testing.T) {
	now := time.Now()
	l := entity.ShareLink{CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	if l.State(now) != entity.ShareActive {
		t.Errorf("state = %s, want active", l.State(now))
	}
	if l.State(now.Add(2*time.Hour)) != entity.ShareExpired {
		t.Error("not expired after its expiry")
	}
	l.RevokedAt = &now
	if l.State(now) != entity.ShareRevoked {
		t.Error("not revoked")
	}
}

func TestShareLinkValidate(t *testing.T) {
	now := time.Now()
	ok := entity.ShareLink{RepositoryID: "r", Detail: entity.ShareSummary, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	tooLong := ok
	tooLong.ExpiresAt = now.Add(31 * 24 * time.Hour)
	noDetail := ok
	noDetail.Detail = ""
	backwards := ok
	backwards.ExpiresAt = now.Add(-time.Minute)
	for name, l := range map[string]entity.ShareLink{"too long": tooLong, "no detail": noDetail, "backwards": backwards} {
		if l.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if d, err := entity.ParseShareDetail(""); err != nil || d != entity.ShareSummary {
		t.Errorf("empty detail = %q, %v; want the summary level", d, err)
	}
	if _, err := entity.ParseShareDetail("everything"); err == nil {
		t.Error("unknown detail accepted")
	}
}
