package ws

import (
	"strings"
	"testing"
)

func TestDecodeIntoKeepsRawEvents(t *testing.T) {
	previous := EnableRawEvents
	EnableRawEvents = true
	t.Cleanup(func() { EnableRawEvents = previous })

	codec := NewCodec(NewOpUnmarshalers())
	buf := NewDecodeBuffer(1 << 10)
	out := make(chan Op, 4)
	for _, payload := range []string{
		`{"op":0,"t":"FIRST","d":{"long":"aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`,
		`{"op":0,"t":"SECOND","d":{"b":1}}`,
	} {
		if err := codec.DecodeInto(t.Context(), strings.NewReader(payload), &buf, out); err != nil {
			t.Fatal(err)
		}
	}
	close(out)

	var raws []string
	for op := range out {
		if raw, ok := op.Data.(*RawEvent); ok {
			raws = append(raws, string(raw.Raw))
		}
	}
	want := []string{`{"long":"aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, `{"b":1}`}
	if len(raws) != len(want) || raws[0] != want[0] || raws[1] != want[1] {
		t.Errorf("raw events = %q, want %q", raws, want)
	}
}
