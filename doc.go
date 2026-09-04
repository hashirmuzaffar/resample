// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

// Package resample converts audio between sample rates using a polyphase
// windowed-sinc filter.
//
// The package has no dependencies outside the standard library.
//
// # Method
//
// Conversion between two rates is a ratio l/m, found by reducing the rates
// through their continued fraction expansion; 44100 to 48000 reduces to
// 160/147. Conceptually the signal is interpolated by l, low-pass filtered,
// and decimated by m. The filter is evaluated in polyphase form, so only the
// taps that land on non-zero samples are computed and the cost per output
// sample is the filter length divided by l.
//
// The prototype filter is a sinc windowed by a Kaiser window, with the shape
// parameter and length chosen from Kaiser's design formulas to meet the
// requested stopband attenuation and transition width. Its cutoff is the
// lower of the two Nyquist frequencies, which simultaneously removes the
// images interpolation creates and the content that would otherwise alias on
// decimation.
//
// # Delay
//
// The prototype length is chosen as 2*d*l+1 so that its centre falls on an
// exact multiple of l. The group delay is then exactly d input samples for
// every polyphase phase, and is compensated internally: output sample k
// corresponds to input time k*m/l with no residual offset.
//
// This matters more than it sounds. A length that leaves a fractional
// residual shifts the output by a fraction of a sample, which is a large
// phase error at high frequencies; converting up and back down again with
// such a filter loses more than 100 dB of round-trip accuracy even though
// every other property of the filter is unchanged.
//
// # Quality
//
// Quality is requested rather than selected from a fixed list: give a
// stopband attenuation and a transition width, and the filter is designed to
// meet them. The Fast, Balanced, HighQuality and Archival presets are
// starting points. Cost scales roughly with attenuation over transition
// width, so halving the transition band doubles the work.
package resample
