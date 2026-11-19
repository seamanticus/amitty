package api

import (
	"testing"

	"github.com/seamanticus/dmwire/utils/httputil"
)

func TestMessagesRangeParamsOmitEmptyBoundaries(t *testing.T) {
	values, err := (&httputil.DefaultSchema{}).Encode(messagesRangeParams{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := values.Encode(), "limit=50"; got != want {
		t.Fatalf("got query %q, want %q", got, want)
	}
}
