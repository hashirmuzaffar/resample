// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

import (
	"math"
	"testing"
)

// analysisBeta shapes the window used to measure output spectra. It must be
// large enough that the window's own sidelobes sit well below the artefacts
// being measured; TestMeasurementFloor pins the resulting floor.
const analysisBeta float64 = 24

// TestAliasRejection is the headline quality measurement. A pure tone is
// resampled and the output spectrum inspected: everything that is not the
// tone is an artefact of the converter, either an image that survived the
// filter or an alias folded back into the band. The worst of them must sit
// below the design attenuation.
func TestAliasRejection(t *testing.T) {
	const n = 1 << 15

	for _, q := range []struct {
		name string
		q    Quality
	}{
		{"Fast", Fast}, {"Balanced", Balanced}, {"HighQuality", HighQuality},
	} {
		for _, c := range []struct{ in, out float64 }{
			{44100, 48000}, {48000, 44100}, {48000, 96000}, {96000, 48000}, {44100, 22050},
		} {
			t.Run(q.name+"/"+rateName(c.in, c.out), func(t *testing.T) {
				// A tone at 30% of the lower Nyquist: comfortably
				// inside the passband for every ratio tested, and
				// not at a harmonic of either rate.
				f := 0.30 * math.Min(c.in, c.out) / 2
				out, err := Resample(tone(n, f, c.in), c.in, c.out, q.q)
				if err != nil {
					t.Fatal(err)
				}
				// Discard the filter's start and end transients.
				trim := len(out) / 8
				spec := spectrumDB(out[trim:len(out)-trim], analysisBeta)

				got, at := sfdr(spec, 8)
				t.Logf("%-12s %6.0f->%-6.0f  SFDR %7.2f dB (worst bin %d)",
					q.name, c.in, c.out, got, at)
				if got < q.q.Attenuation {
					t.Errorf("SFDR %.2f dB is below the design attenuation %.0f dB",
						got, q.q.Attenuation)
				}
			})
		}
	}
}

// TestPassbandFlatness checks that tones across the passband come through at
// unit amplitude. Ripple in the passband is what a listener would hear as
// coloration.
func TestPassbandFlatness(t *testing.T) {
	const n = 1 << 14
	const inRate, outRate = 44100, 48000

	q := HighQuality
	edge := 0.5 * math.Min(inRate, outRate) * (1 - q.Transition)

	var worst float64
	for _, frac := range []float64{0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.98} {
		f := frac * edge
		out, err := Resample(tone(n, f, inRate), inRate, outRate, q)
		if err != nil {
			t.Fatal(err)
		}
		mid := out[len(out)/4 : 3*len(out)/4]
		peak := 0.0
		for _, v := range mid {
			peak = math.Max(peak, math.Abs(v))
		}
		db := 20 * math.Log10(peak)
		worst = math.Max(worst, math.Abs(db))
		t.Logf("%8.1f Hz  (%.0f%% of passband)  gain %+.4f dB", f, frac*100, db)
	}
	if worst > 0.1 {
		t.Errorf("worst passband deviation %.4f dB exceeds 0.1 dB", worst)
	}
}

// TestStopbandRejection feeds a tone above the output Nyquist and checks it
// is removed rather than folded back into the band.
func TestStopbandRejection(t *testing.T) {
	const n = 1 << 15
	const inRate, outRate = 96000, 48000
	q := HighQuality

	// Well above the output Nyquist of 24 kHz: without filtering this
	// would alias to 24000-(30000-24000) = 18 kHz, right in the audible band.
	out, err := Resample(tone(n, 30000, inRate), inRate, outRate, q)
	if err != nil {
		t.Fatal(err)
	}
	trim := len(out) / 8
	x := out[trim : len(out)-trim]

	peak := 0.0
	for _, v := range x {
		peak = math.Max(peak, math.Abs(v))
	}
	db := 20 * math.Log10(peak)
	t.Logf("30 kHz into a 24 kHz Nyquist: residual %.2f dBFS", db)
	if db > -q.Attenuation {
		t.Errorf("residual %.2f dBFS exceeds the design attenuation -%.0f dB", db, q.Attenuation)
	}
}

// TestDCGain checks that a constant input produces the same constant out,
// which is the tightest single check on filter normalisation.
func TestDCGain(t *testing.T) {
	for _, c := range []struct{ in, out float64 }{
		{44100, 48000}, {48000, 44100}, {8000, 16000}, {48000, 8000}, {22050, 44100},
	} {
		// The input must be several filter lengths long, or the
		// centre of the output never sees full filter support and the
		// measurement is of edge effects rather than of DC gain.
		r, err := New(c.in, c.out, HighQuality)
		if err != nil {
			t.Fatal(err)
		}
		in := make([]float64, 8*r.Taps())
		for i := range in {
			in[i] = 1
		}
		out, err := Resample(in, c.in, c.out, HighQuality)
		if err != nil {
			t.Fatal(err)
		}
		mid := out[len(out)/4 : 3*len(out)/4]
		worst := 0.0
		for _, v := range mid {
			worst = math.Max(worst, math.Abs(v-1))
		}
		t.Logf("%6.0f -> %-6.0f  taps %5d  n %6d  worst DC error %.3e", c.in, c.out, r.Taps(), len(in), worst)
		if worst > 1e-6 {
			t.Errorf("%.0f->%.0f: DC error %.3e exceeds 1e-6", c.in, c.out, worst)
		}
	}
}

// TestRoundTrip converts up and back down and compares against the original.
// Only the passband can survive the trip, so the input is band limited.
func TestRoundTrip(t *testing.T) {
	const n = 1 << 14
	const a, b = 44100, 48000
	q := HighQuality

	// A chirp sweeping the passband, which is not periodic and so cannot
	// hide a misalignment behind its own repetition.
	in := make([]float64, n)
	dur := float64(n) / a
	for i := range in {
		tt := float64(i) / a
		in[i] = 0.7 * math.Sin(2*math.Pi*(200*tt+(15000-200)/2*tt*tt/dur))
	}

	up, err := Resample(in, a, b, q)
	if err != nil {
		t.Fatal(err)
	}
	down, err := Resample(up, b, a, q)
	if err != nil {
		t.Fatal(err)
	}

	nCmp := min(len(in), len(down))
	lo, hi := nCmp/8, nCmp-nCmp/8
	var num, den float64
	for i := lo; i < hi; i++ {
		d := down[i] - in[i]
		num += d * d
		den += in[i] * in[i]
	}
	snr := 10 * math.Log10(den/num)
	t.Logf("round trip %d->%d->%d  SNR %.2f dB over %d samples", a, b, a, snr, hi-lo)
	if snr < q.Attenuation {
		t.Errorf("round-trip SNR %.2f dB is below the design attenuation %.0f dB", snr, q.Attenuation)
	}
}

func rateName(in, out float64) string {
	return itoa(int(in)) + "to" + itoa(int(out))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
