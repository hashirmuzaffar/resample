// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

import (
	"fmt"
	"math"
)

// Quality describes the anti-imaging and anti-aliasing filter used for a
// conversion. Larger attenuation and narrower transition bands cost
// proportionally more work per sample.
type Quality struct {
	// Attenuation is the minimum stopband attenuation in dB. Images and
	// aliases are suppressed by at least this much.
	Attenuation float64

	// Transition is the width of the filter's transition band as a
	// fraction of the lower of the two Nyquist frequencies. A value of
	// 0.02 keeps the response flat to 99% of the usable band.
	Transition float64
}

// Preset qualities. The names describe the intended use rather than any
// particular other implementation.
var (
	// Fast is suitable for previews and metering: 80 dB of image
	// rejection, which is below 16-bit noise floor at full scale.
	Fast = Quality{Attenuation: 80, Transition: 0.10}

	// Balanced is transparent for 16-bit material.
	Balanced = Quality{Attenuation: 100, Transition: 0.05}

	// HighQuality exceeds 20-bit dynamic range and keeps the passband
	// flat to within 0.02 of Nyquist.
	HighQuality = Quality{Attenuation: 120, Transition: 0.02}

	// Archival is beyond 24-bit; the filter is long and the cost is real.
	Archival = Quality{Attenuation: 150, Transition: 0.01}
)

func (q Quality) validate() error {
	if q.Attenuation < 20 || q.Attenuation > 200 {
		return fmt.Errorf("resample: attenuation %g dB out of range [20, 200]", q.Attenuation)
	}
	if q.Transition <= 0 || q.Transition >= 1 {
		return fmt.Errorf("resample: transition %g out of range (0, 1)", q.Transition)
	}
	return nil
}

// designPolyphase builds the prototype low-pass filter for conversion by
// l/m and splits it into l phases.
//
// The filter runs at the interpolated rate l*fs. Its cutoff is the lower of
// the two Nyquist frequencies, which is what simultaneously removes the
// images introduced by upsampling and the content that would alias on
// downsampling. The prototype is a sinc windowed by a Kaiser window, scaled
// by l so that unit input gives unit output.
func designPolyphase(l, m int, q Quality) (phases [][]float64, tapsPerPhase, delay int) {
	// Normalised cutoff at the interpolated rate, in cycles per sample.
	// 0.5/max(l,m) is the lower of the two Nyquist frequencies there.
	nyq := 0.5 / float64(max(l, m))
	cutoff := nyq * (1 - q.Transition/2)
	transition := nyq * q.Transition

	n := kaiserLength(q.Attenuation, transition)

	// Choose the prototype length as 2*d*l+1 so that its centre sits at
	// index d*l, an exact multiple of l. The group delay is then exactly
	// d input samples for every phase; any other length leaves a
	// fractional residual that shows up as a sub-sample shift, which is a
	// large phase error at high frequencies and ruins round trips.
	d := (n - 1 + 2*l - 1) / (2 * l)
	if d < 1 {
		d = 1
	}
	tapsPerPhase = 2*d + 1
	n = 2*d*l + 1

	beta := kaiserBeta(q.Attenuation)
	win := make([]float64, n)
	kaiserWindow(win, beta)

	// h is padded to a whole number of phases; the tail beyond the
	// prototype stays zero.
	h := make([]float64, tapsPerPhase*l)
	center := float64(d * l)
	for i := 0; i < n; i++ {
		h[i] = 2 * cutoff * sinc(2*cutoff*(float64(i)-center)) * win[i]
	}

	// Normalise so that the interpolated-and-filtered signal keeps unit
	// amplitude. Each phase must sum to one; scaling the whole prototype
	// by l achieves that when the design is symmetric, but normalising
	// against the measured DC gain is exact regardless of rounding.
	var dc float64
	for _, v := range h {
		dc += v
	}
	scale := float64(l) / dc
	for i := range h {
		h[i] *= scale
	}

	phases = make([][]float64, l)
	for p := range phases {
		ph := make([]float64, tapsPerPhase)
		for j := range ph {
			ph[j] = h[p+j*l]
		}
		phases[p] = ph
	}
	return phases, tapsPerPhase, d
}

// gcd returns the greatest common divisor of a and b.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// ratio reduces inRate:outRate to the smallest integer pair l:m such that
// resampling by l/m converts between them.
//
// Rates are matched to a resolution of one part in 1e9, which covers every
// standard audio rate exactly and keeps l and m small for them; 44100 to
// 48000 reduces to 160/147.
func ratio(inRate, outRate float64) (l, m int, err error) {
	if !(inRate > 0) || !(outRate > 0) || math.IsInf(inRate, 0) || math.IsInf(outRate, 0) {
		return 0, 0, fmt.Errorf("resample: rates must be finite and positive, got %g and %g", inRate, outRate)
	}

	const maxDen = 1 << 20
	l, m = rationalise(outRate/inRate, maxDen)
	if l <= 0 || m <= 0 {
		return 0, 0, fmt.Errorf("resample: cannot represent ratio %g/%g", outRate, inRate)
	}
	return l, m, nil
}

// rationalise approximates x by p/q with q no larger than maxDen, using the
// continued fraction expansion. It returns the convergent, which is the best
// rational approximation with a denominator that size.
func rationalise(x float64, maxDen int) (p, q int) {
	// Continued fraction: track the last two convergents.
	p0, q0 := 0, 1
	p1, q1 := 1, 0
	f := x
	for i := 0; i < 64; i++ {
		a := math.Floor(f)
		ai := int(a)
		np := ai*p1 + p0
		nq := ai*q1 + q0
		if nq > maxDen || np > maxDen {
			break
		}
		p0, q0 = p1, q1
		p1, q1 = np, nq

		frac := f - a
		if frac < 1e-12 {
			break
		}
		f = 1 / frac
	}
	if q1 == 0 {
		return 0, 0
	}
	g := gcd(p1, q1)
	return p1 / g, q1 / g
}
