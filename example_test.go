// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample_test

import (
	"fmt"
	"math"

	"github.com/hashirmuzaffar/resample"
)

func ExampleResample() {
	// One second of a 1 kHz tone at 44.1 kHz.
	in := make([]float64, 44100)
	for i := range in {
		in[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / 44100)
	}

	out, err := resample.Resample(in, 44100, 48000, resample.HighQuality)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d samples in, %d samples out\n", len(in), len(out))

	// Output:
	// 44100 samples in, 48427 samples out
}

func ExampleNew_streaming() {
	r, err := resample.New(44100, 48000, resample.Balanced)
	if err != nil {
		panic(err)
	}
	l, m := r.Ratio()
	fmt.Printf("ratio %d/%d, %d taps per output sample, %d samples of delay\n",
		l, m, r.Taps(), r.Delay())

	// Feed the stream in blocks; output arrives as it becomes available.
	var total int
	block := make([]float64, 1024)
	for i := 0; i < 43; i++ {
		total += len(r.Write(block))
	}
	total += len(r.Flush())
	fmt.Printf("%d input samples produced %d output samples\n", 43*1024, total)

	// Output:
	// ratio 160/147, 259 taps per output sample, 129 samples of delay
	// 44032 input samples produced 48068 output samples
}
