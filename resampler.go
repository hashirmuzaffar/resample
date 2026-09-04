// Copyright 2026 Hashir Muzaffar. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license
// that can be found in the LICENSE file.

package resample

// Resampler converts a stream of samples from one rate to another. It is
// not safe for concurrent use.
//
// Feed input with Write and take output from the returned slice; call Flush
// once at the end of the stream to drain the filter's tail.
type Resampler struct {
	l, m   int
	phases [][]float64
	taps   int

	// buf holds the input samples still needed by the filter, followed by
	// any newly written samples. origin is the index, in the whole input
	// stream, of buf[0].
	buf    []float64
	origin int64

	// k is the index of the next output sample to produce.
	k int64

	// shift is the filter's group delay rounded to whole input samples.
	// Output k reads input around index k*m/l + shift, which cancels the
	// delay so that output time lines up with input time.
	shift int64

	flushed bool
}

// New returns a Resampler converting from inRate to outRate.
//
// The rates are used only as a ratio, so New(44100, 48000, q) and
// New(147, 160, q) are the same converter.
func New(inRate, outRate float64, q Quality) (*Resampler, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	l, m, err := ratio(inRate, outRate)
	if err != nil {
		return nil, err
	}

	// The prototype is built so its centre falls on an exact multiple of
	// l, which makes the group delay a whole number of input samples for
	// every phase and leaves nothing to compensate afterwards.
	phases, taps, delay := designPolyphase(l, m, q)

	return &Resampler{
		l:      l,
		m:      m,
		phases: phases,
		taps:   taps,
		shift:  int64(delay),
	}, nil
}

// Ratio reports the reduced conversion ratio: output rate over input rate is
// l/m.
func (r *Resampler) Ratio() (l, m int) { return r.l, r.m }

// Taps reports the number of multiply-accumulate operations per output
// sample.
func (r *Resampler) Taps() int { return r.taps }

// Delay reports the filter's group delay in input samples. It is
// compensated internally, so output sample k corresponds to input time
// k*m/l with no residual offset; the value is exposed for callers that need
// to reason about latency.
func (r *Resampler) Delay() int { return int(r.shift) }

// Write consumes in and returns the output samples that became available.
// The returned slice is only valid until the next call.
func (r *Resampler) Write(in []float64) []float64 {
	r.buf = append(r.buf, in...)
	return r.emit(false)
}

// Flush returns the remaining output, feeding zeros through the filter to
// drain its tail. After Flush the Resampler must not be used again.
func (r *Resampler) Flush() []float64 {
	if r.flushed {
		return nil
	}
	r.flushed = true
	// Enough zeros to push the last real sample through the filter.
	r.buf = append(r.buf, make([]float64, r.taps)...)
	return r.emit(true)
}

// OutputLen reports how many output samples a complete input of n samples
// produces, including the flush.
func (r *Resampler) OutputLen(n int) int {
	if n <= 0 {
		return 0
	}
	// A signal of n input samples spans n*l/m output samples, rounded up.
	return (n*r.l + r.m - 1) / r.m
}

// emit produces every output sample whose filter support is now covered by
// buf. When draining, the tail zeros are already in buf.
func (r *Resampler) emit(drain bool) []float64 {
	// Highest input index available, in whole-stream coordinates.
	avail := r.origin + int64(len(r.buf)) - 1

	var out []float64
	for {
		q := (r.k*int64(r.m))/int64(r.l) + r.shift
		if q > avail {
			break
		}
		p := int((r.k * int64(r.m)) % int64(r.l))

		ph := r.phases[p]
		// y[k] = sum_j ph[j] * x[q-j]
		base := q - r.origin // index of x[q] within buf
		var acc float64
		for j := 0; j < len(ph); j++ {
			i := base - int64(j)
			if i < 0 {
				break // before the start of the stream: zero
			}
			acc += ph[j] * r.buf[i]
		}
		out = append(out, acc)
		r.k++
	}

	// Drop input that no future output can reach. The oldest index still
	// needed is q_next - taps + 1.
	if !drain {
		qNext := (r.k*int64(r.m))/int64(r.l) + r.shift
		keepFrom := qNext - int64(r.taps) + 1
		if keepFrom > r.origin {
			cut := keepFrom - r.origin
			if cut > int64(len(r.buf)) {
				cut = int64(len(r.buf))
			}
			r.buf = append(r.buf[:0], r.buf[cut:]...)
			r.origin += cut
		}
	}
	return out
}

// Resample converts in from inRate to outRate in one call.
func Resample(in []float64, inRate, outRate float64, q Quality) ([]float64, error) {
	r, err := New(inRate, outRate, q)
	if err != nil {
		return nil, err
	}
	out := make([]float64, 0, r.OutputLen(len(in)))
	out = append(out, r.Write(in)...)
	out = append(out, r.Flush()...)
	return out, nil
}
