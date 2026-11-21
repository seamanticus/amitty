// Package json allows for different implementations of JSON serializing, as
// well as extra optional types needed.
package json

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
)

// Options are the options every encode and decode uses: encoding/json's v1 behaviour on encoding/json/v2. Tools that must decode as amitty does use them too.
func Options() jsonv2.Options { return jsonv1.DefaultOptionsV1() }

type Driver interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error

	DecodeStream(r io.Reader, v any) error
	EncodeStream(w io.Writer, v any) error
}

type DefaultDriver struct{}

func (d DefaultDriver) Marshal(v any) ([]byte, error) {
	return jsonv2.Marshal(v, Options())
}

func (d DefaultDriver) Unmarshal(data []byte, v any) error {
	return jsonv2.Unmarshal(data, v, Options())
}

func (d DefaultDriver) DecodeStream(r io.Reader, v any) error {
	return jsonv2.UnmarshalDecode(jsontext.NewDecoder(r), v, Options())
}

func (d DefaultDriver) EncodeStream(w io.Writer, v any) error {
	return jsonv2.MarshalEncode(jsontext.NewEncoder(w), v, Options())
}

// Default is the default JSON driver, which uses encoding/json/v2.
var Default Driver = DefaultDriver{}

// Marshal uses the default driver.
func Marshal(v any) ([]byte, error) {
	return Default.Marshal(v)
}

// Unmarshal uses the default driver.
func Unmarshal(data []byte, v any) error {
	return Default.Unmarshal(data, v)
}

// DecodeStream uses the default driver.
func DecodeStream(r io.Reader, v any) error {
	return Default.DecodeStream(r, v)
}

// EncodeStream uses the default driver.
func EncodeStream(w io.Writer, v any) error {
	return Default.EncodeStream(w, v)
}
