// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

import "math"

// besselI0 evaluates the zeroth order modified Bessel function of the first
// kind,
//
//	I0(x) = sum_{k>=0} ((x/2)^k / k!)^2
//
// The series is summed directly. Each term is formed from the previous one,
// so no factorial is ever evaluated, and the loop stops once a term can no
// longer change the sum.
func besselI0(x float64) float64 {
	half := x / 2
	sum, term := 1.0, 1.0
	for k := 1; k < 200; k++ {
		// t_k = t_{k-1} * (x/2 / k)^2
		r := half / float64(k)
		term *= r * r
		sum += term
		if term < sum*1e-17 {
			break
		}
	}
	return sum
}

// kaiserWindow fills w with an n point Kaiser window of shape beta. The
// window is symmetric, so w[i] == w[n-1-i].
func kaiserWindow(w []float64, beta float64) {
	n := len(w)
	if n == 1 {
		w[0] = 1
		return
	}
	den := besselI0(beta)
	m := float64(n - 1)
	for i := range w {
		r := 2*float64(i)/m - 1 // -1 .. 1
		w[i] = besselI0(beta*math.Sqrt(math.Max(0, 1-r*r))) / den
	}
}

// kaiserBeta returns the Kaiser shape parameter giving at least the
// requested stopband attenuation in dB, using Kaiser's empirical formula.
func kaiserBeta(attenDB float64) float64 {
	switch {
	case attenDB > 50:
		return 0.1102 * (attenDB - 8.7)
	case attenDB >= 21:
		return 0.5842*math.Pow(attenDB-21, 0.4) + 0.07886*(attenDB-21)
	default:
		return 0
	}
}

// kaiserLength returns the filter length needed to reach attenDB of stopband
// attenuation with a transition band of width transition, expressed in
// cycles per sample. The result is Kaiser's estimate rounded up to an odd
// length so that the filter has an exact integer group delay.
func kaiserLength(attenDB, transition float64) int {
	if transition <= 0 {
		panic("resample: transition width must be positive")
	}
	n := int(math.Ceil((attenDB - 8) / (2.285 * 2 * math.Pi * transition)))
	if n < 1 {
		n = 1
	}
	if n%2 == 0 {
		n++
	}
	return n
}

// sinc returns sin(pi*x)/(pi*x), with the removable singularity at zero
// filled in.
func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	px := math.Pi * x
	return math.Sin(px) / px
}
