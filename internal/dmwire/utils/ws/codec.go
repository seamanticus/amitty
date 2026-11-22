package ws

import (
	"context"
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"

	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
)

// Codec holds the codec states for Websocket implementations to share with the
// manager. It is used internally in the Websocket and the Connection
// implementation.
type Codec struct {
	Unmarshalers OpUnmarshalers
	Headers      http.Header
	// Log, if set, is given every frame sent or received, before it is decoded.
	Log func(Frame)
}

// Frame is a gateway payload as it crossed the wire. Data is only valid during the call to Codec.Log.
type Frame struct {
	Sent     bool
	Code     OpCode
	Type     EventType
	Sequence int64
	Data     []byte
}

// NewCodec creates a new default Codec instance.
func NewCodec(unmarshalers OpUnmarshalers) Codec {
	return Codec{
		Unmarshalers: unmarshalers,
		Headers: http.Header{
			"Accept-Encoding": {"zlib"},
		},
	}
}

type codecOp struct {
	Op
	Data json.Raw `json:"d,omitempty"`
	// Seq is what the voice gateway calls Sequence.
	Seq int64 `json:"seq"`
}

const maxSharedBufferSize = 1 << 15 // 32KB

// DecodeBuffer boxes a byte slice to provide a shared and thread-unsafe buffer.
// It is used internally and should only be handled around as an opaque thing.
type DecodeBuffer struct {
	buf []byte
}

// NewDecodeBuffer creates a new preallocated DecodeBuffer.
func NewDecodeBuffer(cap int) DecodeBuffer {
	if cap > maxSharedBufferSize {
		cap = maxSharedBufferSize
	}

	return DecodeBuffer{
		buf: make([]byte, 0, cap),
	}
}

// DecodeInto reads the given reader and decodes it into the Op out channel.
//
// buf is optional.
func (c Codec) DecodeInto(ctx context.Context, r io.Reader, buf *DecodeBuffer, out chan<- Op) error {
	var op codecOp
	op.Data = json.Raw(buf.buf)

	if err := json.DecodeStream(r, &op); err != nil {
		return c.send(ctx, out, newErrOp(err, "cannot read JSON stream"))
	}

	if op.Seq != 0 {
		op.Sequence = op.Seq
	}

	if c.Log != nil {
		c.Log(Frame{Code: op.Code, Type: op.Type, Sequence: op.Sequence, Data: op.Data})
	}

	// buf isn't grown from here out. Set it back right now. If Data hasn't been
	// grown, then this will just set buf back to what it was.
	if cap(op.Data) < maxSharedBufferSize {
		buf.buf = op.Data[:0]
	}

	fn := c.Unmarshalers.Lookup(op.Code, op.Type)
	if fn == nil {
		err := UnknownEventError{
			Op:   op.Code,
			Type: op.Type,
		}
		return c.send(ctx, out, newErrOp(err, ""))
	}

	op.Op.Data = fn()
	if err := op.Data.UnmarshalTo(op.Op.Data); err != nil {
		return c.send(ctx, out, newErrOp(err, fmt.Sprintf(
			"cannot unmarshal JSON event (op %d type %q) from gateway",
			op.Code, op.Type,
		)))
	}

	return c.send(ctx, out, op.Op)
}

// decodeBinary decodes b if it is a binary message, which is a sequence number,
// an Op code and an event that is an encoding.BinaryUnmarshaler. Otherwise, b
// is compressed JSON and false is returned.
func (c Codec) decodeBinary(b []byte) (Op, bool) {
	if len(b) < 3 {
		return Op{}, false
	}
	fn := c.Unmarshalers.Lookup(OpCode(b[2]), "")
	if fn == nil {
		return Op{}, false
	}
	data := fn()
	event, ok := data.(encoding.BinaryUnmarshaler)
	if !ok {
		return Op{}, false
	}
	if err := event.UnmarshalBinary(b[3:]); err != nil {
		return newErrOp(err, fmt.Sprintf("cannot unmarshal binary event (op %d) from gateway", b[2])), true
	}
	if c.Log != nil {
		c.Log(Frame{Code: data.Op(), Sequence: int64(binary.BigEndian.Uint16(b)), Data: b[3:]})
	}
	return Op{Code: data.Op(), Data: data, Sequence: int64(binary.BigEndian.Uint16(b))}, true
}

// logSent gives c.Log the frame b that Conn wrote, a binary message when binaryMessage is set.
func (c Codec) logSent(binaryMessage bool, b []byte) {
	if c.Log == nil {
		return
	}
	frame := Frame{Sent: true, Data: b}
	if binaryMessage {
		if len(b) > 0 {
			frame.Code, frame.Data = OpCode(b[0]), b[1:]
		}
	} else {
		var op codecOp
		if err := json.Unmarshal(b, &op); err == nil {
			frame.Code, frame.Type, frame.Data = op.Code, op.Type, op.Data
		}
	}
	c.Log(frame)
}

func (c *Codec) send(ctx context.Context, ch chan<- Op, op Op) error {
	select {
	case ch <- op:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newErrOp(err error, wrap string) Op {
	if wrap != "" {
		err = fmt.Errorf("%s: %w", wrap, err)
	}

	ev := &BackgroundErrorEvent{
		Err: err,
	}

	return Op{
		Code: ev.Op(),
		Type: ev.EventType(),
		Data: ev,
	}
}
