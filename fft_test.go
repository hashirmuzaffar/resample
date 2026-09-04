// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

import "math"

// A small radix-2 FFT lives here rather than in the package proper: the
// resampler itself needs no transform, and keeping it in the tests lets the
// package stay dependency free.

// fft computes the in-place decimation-in-time FFT of re/im, whose length
// must be a power of two.
func fft(re, im []float64) {
	n := len(re)
	if n&(n-1) != 0 {
		panic("fft: length must be a power of two")
	}
	// Bit-reversal permutation.
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wr, wi := math.Cos(ang), math.Sin(ang)
		for i := 0; i < n; i += length {
			cr, ci := 1.0, 0.0
			for j := 0; j < length/2; j++ {
				ur, ui := re[i+j], im[i+j]
				vr := re[i+j+length/2]*cr - im[i+j+length/2]*ci
				vi := re[i+j+length/2]*ci + im[i+j+length/2]*cr
				re[i+j], im[i+j] = ur+vr, ui+vi
				re[i+j+length/2], im[i+j+length/2] = ur-vr, ui-vi
				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
}

// spectrumDB returns the magnitude spectrum of x in dB, normalised so that
// the largest bin is 0 dB. x is windowed first with a Kaiser window of the
// given beta; a high beta is needed so that the window's own sidelobes stay
// far below the artefacts being measured.
func spectrumDB(x []float64, beta float64) []float64 {
	n := 1
	for n < len(x) {
		n <<= 1
	}
	if n > len(x) {
		n >>= 1 // use the largest power of two that fits, no zero padding
	}
	x = x[:n]

	w := make([]float64, n)
	kaiserWindow(w, beta)

	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = x[i] * w[i]
	}
	fft(re, im)

	half := n/2 + 1
	mag := make([]float64, half)
	max := 0.0
	for i := 0; i < half; i++ {
		mag[i] = math.Hypot(re[i], im[i])
		if mag[i] > max {
			max = mag[i]
		}
	}
	out := make([]float64, half)
	for i, v := range mag {
		if v <= 0 {
			out[i] = -400
			continue
		}
		out[i] = 20 * math.Log10(v/max)
	}
	return out
}

// sfdr returns the spurious-free dynamic range of a signal expected to be a
// single tone: the level of the largest spectral component that is not part
// of the tone's own main lobe, in dB below the tone. skirt is the number of
// bins either side of the peak treated as belonging to the tone.
func sfdr(spec []float64, skirt int) (db float64, atBin int) {
	peak := 0
	for i, v := range spec {
		if v > spec[peak] {
			peak = i
		}
	}
	worst, at := -400.0, -1
	for i, v := range spec {
		if i > peak-skirt && i < peak+skirt {
			continue
		}
		// Ignore the DC bin and its immediate neighbours: any tiny
		// offset shows up there and is not an aliasing artefact.
		if i < skirt {
			continue
		}
		if v > worst {
			worst, at = v, i
		}
	}
	return -worst, at
}

// tone generates n samples of a unit sine at freq Hz sampled at rate Hz.
func tone(n int, freq, rate float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * freq * float64(i) / rate)
	}
	return x
}
