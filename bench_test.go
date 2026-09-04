// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

import "testing"

func benchOne(b *testing.B, in, out float64, q Quality) {
	b.Helper()
	const seconds = 1
	x := tone(int(in*seconds), 1000, in)
	r, err := New(in, out, q)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(r.Taps()), "taps/sample")
	b.SetBytes(int64(len(x) * 8))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rr, _ := New(in, out, q)
		rr.Write(x)
		rr.Flush()
	}
}

func BenchmarkFast44to48(b *testing.B)        { benchOne(b, 44100, 48000, Fast) }
func BenchmarkBalanced44to48(b *testing.B)    { benchOne(b, 44100, 48000, Balanced) }
func BenchmarkHighQuality44to48(b *testing.B) { benchOne(b, 44100, 48000, HighQuality) }
func BenchmarkHighQuality48to44(b *testing.B) { benchOne(b, 48000, 44100, HighQuality) }
func BenchmarkHighQuality48to16(b *testing.B) { benchOne(b, 48000, 16000, HighQuality) }

// BenchmarkSteadyState excludes filter design, measuring only the cost of
// pushing samples through an already built resampler.
func BenchmarkSteadyState44to48(b *testing.B) {
	x := tone(44100, 1000, 44100)
	r, err := New(44100, 48000, HighQuality)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(x) * 8))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.Write(x)
	}
}

func BenchmarkDesign44to48(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := New(44100, 48000, HighQuality); err != nil {
			b.Fatal(err)
		}
	}
}
