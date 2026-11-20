//go:build ignore

// Writes the notification sounds to sounds/: short synthesised tones, the same on every run. Run with go generate.
package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
)

const rate = 22050

// note is a tone starting at start seconds: freq in Hz with partials (ratio, gain), decaying by decay per second, sweeping to sweep Hz if set.
type note struct {
	start, length, freq, decay, sweep float64
	partials                          [][2]float64
}

// pure is a sine with a soft octave overtone.
var pure = [][2]float64{{1, 1}, {2, 0.25}}

// bellPartials are a struck bell's inharmonic overtones.
var bellPartials = [][2]float64{{1, 1}, {2.0, 0.5}, {3.0, 0.3}, {4.2, 0.2}, {5.4, 0.12}}

func render(notes []note) []float64 {
	var end float64
	for _, n := range notes {
		end = max(end, n.start+n.length)
	}
	out := make([]float64, int(end*rate)+1)
	for _, n := range notes {
		first, count := int(n.start*rate), int(n.length*rate)
		phase := 0.0
		for i := range count {
			t := float64(i) / rate
			freq := n.freq
			if n.sweep != 0 {
				freq = n.freq + (n.sweep-n.freq)*t/n.length
			}
			phase += 2 * math.Pi * freq / rate
			attack := min(1, t/0.004)
			release := min(1, (n.length-t)/0.01)
			var v float64
			for _, p := range n.partials {
				v += p[1] * math.Sin(phase*p[0])
			}
			out[first+i] += v * math.Exp(-t*n.decay) * attack * release
		}
	}
	return out
}

// pluck is a Karplus-Strong string at freq, seeded so every run gives the same noise.
func pluck(freq, length float64) []float64 {
	r := rand.New(rand.NewPCG(1, 2))
	ring := make([]float64, int(rate/freq))
	for i := range ring {
		ring[i] = r.Float64()*2 - 1
	}
	out := make([]float64, int(length*rate))
	for i := range out {
		j := i % len(ring)
		out[i] = ring[j]
		ring[j] = 0.996 * (ring[j] + ring[(j+1)%len(ring)]) / 2
	}
	return out
}

// knock is two short low thumps, as on a door.
func knock() []float64 {
	thump := func(at float64) note {
		return note{start: at, length: 0.09, freq: 220, sweep: 110, decay: 40, partials: [][2]float64{{1, 1}, {2.3, 0.4}}}
	}
	return render([]note{thump(0), thump(0.14)})
}

func sounds() map[string][]float64 {
	return map[string][]float64{
		// The first default: E6 then A6.
		"chime": render([]note{{0, 0.12, 1318.51, 9, 0, pure}, {0.12, 0.30, 1760, 9, 0, pure}}),
		"ding":  render([]note{{0, 0.6, 880, 6, 0, bellPartials}}),
		"dong":  render([]note{{0, 0.2, 880, 7, 0, bellPartials}, {0.2, 0.45, 659.25, 6, 0, bellPartials}}),
		// Two falling drops, each long enough to be heard.
		"pop":    render([]note{{0, 0.14, 1200, 18, 500, pure}, {0.12, 0.18, 1000, 14, 420, pure}}),
		"blip":   render([]note{{0, 0.12, 1046.5, 10, 0, [][2]float64{{1, 1}, {3, 0.33}, {5, 0.2}}}, {0.13, 0.16, 1396.91, 9, 0, [][2]float64{{1, 1}, {3, 0.33}, {5, 0.2}}}}),
		"bubble": render([]note{{0, 0.16, 500, 9, 1300, pure}, {0.14, 0.2, 650, 8, 1600, pure}}),
		// C6, E6 and G6 in turn.
		"triad":   render([]note{{0, 0.3, 1046.5, 8, 0, pure}, {0.08, 0.3, 1318.51, 8, 0, pure}, {0.16, 0.35, 1567.98, 8, 0, pure}}),
		"marimba": render([]note{{0, 0.35, 698.46, 14, 0, [][2]float64{{1, 1}, {4, 0.3}, {10, 0.05}}}}),
		"glass":   render([]note{{0, 0.5, 2093, 8, 0, [][2]float64{{1, 1}, {2.76, 0.4}, {5.4, 0.15}}}}),
		"knock":   knock(),
		"pluck":   pluck(783.99, 0.5),
		// A pentatonic run up from D6.
		"sparkle": render([]note{{0, 0.15, 1174.66, 12, 0, pure}, {0.06, 0.15, 1318.51, 12, 0, pure}, {0.12, 0.15, 1567.98, 12, 0, pure}, {0.18, 0.3, 1760, 10, 0, pure}}),
		"bell":    render([]note{{0, 0.8, 523.25, 4, 0, bellPartials}}),
		// B5 then E6, quickly.
		"coin": render([]note{{0, 0.07, 987.77, 4, 0, [][2]float64{{1, 1}, {3, 0.2}}}, {0.07, 0.3, 1318.51, 8, 0, [][2]float64{{1, 1}, {3, 0.2}}}}),
	}
}

// leadIn is the silence before each sound, which a sound server waking its output from suspend may swallow in place of the sound.
const leadIn = 0.08

// wav encodes samples as 16-bit mono PCM after leadIn of silence, scaled to the same average loudness (RMS) as the others, with the peak kept below full scale.
func wav(samples []float64) []byte {
	var peak, energy float64
	for _, s := range samples {
		peak = max(peak, math.Abs(s))
		energy += s * s
	}
	rms := math.Sqrt(energy / float64(len(samples)))
	// -18 dBFS on average, at most -1 dBFS at the peak.
	gain := min(math.Pow(10, -18.0/20)/rms, math.Pow(10, -1.0/20)/peak)
	samples = append(make([]float64, int(leadIn*rate)), samples...)
	var data bytes.Buffer
	for _, s := range samples {
		_ = binary.Write(&data, binary.LittleEndian, int16(s*gain*math.MaxInt16))
	}
	var out bytes.Buffer
	write := func(v any) { _ = binary.Write(&out, binary.LittleEndian, v) }
	out.WriteString("RIFF")
	write(uint32(36 + data.Len()))
	out.WriteString("WAVEfmt ")
	write(uint32(16))
	write(uint16(1)) // PCM
	write(uint16(1)) // mono
	write(uint32(rate))
	write(uint32(rate * 2))
	write(uint16(2))
	write(uint16(16))
	out.WriteString("data")
	write(uint32(data.Len()))
	out.Write(data.Bytes())
	return out.Bytes()
}

func main() {
	if err := os.MkdirAll("sounds", 0o755); err != nil {
		log.Fatal(err)
	}
	for name, samples := range sounds() {
		if err := os.WriteFile(filepath.Join("sounds", name+".wav"), wav(samples), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
