//go:build ignore

// Writes notification.wav: a short two-note chime, the default notification sound. Run with go generate.
package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"os"
)

const rate = 22050

func main() {
	var samples []int16
	// E6 then A6, each fading out, the second a little longer.
	for _, note := range []struct {
		freq    float64
		seconds float64
	}{{1318.51, 0.12}, {1760.00, 0.30}} {
		n := int(note.seconds * rate)
		for i := range n {
			t := float64(i) / rate
			envelope := math.Exp(-t * 9)
			attack := min(1, float64(i)/(0.004*rate))
			// A soft overtone an octave up rounds the tone.
			v := math.Sin(2*math.Pi*note.freq*t) + 0.25*math.Sin(4*math.Pi*note.freq*t)
			samples = append(samples, int16(v/1.25*envelope*attack*0.5*math.MaxInt16))
		}
	}

	var data bytes.Buffer
	for _, s := range samples {
		_ = binary.Write(&data, binary.LittleEndian, s)
	}
	var wav bytes.Buffer
	write := func(v any) { _ = binary.Write(&wav, binary.LittleEndian, v) }
	wav.WriteString("RIFF")
	write(uint32(36 + data.Len()))
	wav.WriteString("WAVEfmt ")
	write(uint32(16))
	write(uint16(1)) // PCM
	write(uint16(1)) // mono
	write(uint32(rate))
	write(uint32(rate * 2))
	write(uint16(2))
	write(uint16(16))
	wav.WriteString("data")
	write(uint32(data.Len()))
	wav.Write(data.Bytes())

	if err := os.WriteFile("notification.wav", wav.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}
