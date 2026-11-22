package ws

import (
	"strings"
	"testing"
)

func TestCodecLogsFrames(t *testing.T) {
	var frames []Frame
	codec := NewCodec(NewOpUnmarshalers())
	codec.Log = func(f Frame) {
		// Data is only valid during the call.
		f.Data = []byte(string(f.Data))
		frames = append(frames, f)
	}
	buf := NewDecodeBuffer(1 << 10)
	out := make(chan Op, 4)
	for _, payload := range []string{
		`{"op":0,"t":"FIRST","s":3,"d":{"long":"aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`,
		`{"op":11}`,
	} {
		if err := codec.DecodeInto(t.Context(), strings.NewReader(payload), &buf, out); err != nil {
			t.Fatal(err)
		}
	}
	codec.logSent(false, []byte(`{"op":1,"d":3}`))
	codec.logSent(true, []byte{37, 'x'})

	want := []Frame{
		{Code: 0, Type: "FIRST", Sequence: 3, Data: []byte(`{"long":"aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)},
		{Code: 11, Data: []byte(``)},
		{Sent: true, Code: 1, Data: []byte(`3`)},
		{Sent: true, Code: 37, Data: []byte(`x`)},
	}
	if len(frames) != len(want) {
		t.Fatalf("frames = %+v, want %+v", frames, want)
	}
	for i := range want {
		got, w := frames[i], want[i]
		if got.Sent != w.Sent || got.Code != w.Code || got.Type != w.Type || got.Sequence != w.Sequence || string(got.Data) != string(w.Data) {
			t.Errorf("frame %d = %+v %s, want %+v %s", i, got, got.Data, w, w.Data)
		}
	}
}
