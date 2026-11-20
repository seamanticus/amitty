package chat

import (
	"testing"
	"time"
)

// evidence: witnesses
func TestOutreachAllow(t *testing.T) {
	var o outreach
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range outreachLimit {
		if !o.allow(now.Add(time.Duration(i) * time.Minute)) {
			t.Fatalf("outreach %d refused under the limit", i+1)
		}
	}
	if o.allow(now.Add(outreachWindow - time.Second)) {
		t.Fatal("outreach over the limit within the window was allowed")
	}
	if !o.allow(now.Add(outreachWindow)) {
		t.Fatal("the first outreach did not leave the window")
	}
	if len(o) != outreachLimit {
		t.Fatalf("%d kept, want %d", len(o), outreachLimit)
	}
}
