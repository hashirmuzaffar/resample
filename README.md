# resample

High-quality audio sample rate conversion for Go. Polyphase, windowed sinc,
no dependencies outside the standard library.

Every audio pipeline needs one of these and Go doesn't have a good one. This
is a resampler whose quality claims are *measured* rather than asserted. The
test suite reports alias rejection, passband flatness and round-trip SNR in
decibels, and fails if any of them misses the design target.

```go
out, err := resample.Resample(in, 44100, 48000, resample.HighQuality)
```

Or streaming, for a pipeline that gets audio in blocks:

```go
r, _ := resample.New(44100, 48000, resample.Balanced)
for block := range blocks {
    process(r.Write(block))
}
process(r.Flush())
```

## Measured quality

Worst-case spurious-free dynamic range: a pure tone is resampled and
everything in the output that isn't the tone, meaning surviving images and folded
aliases, is measured against it.

| conversion | Fast (80 dB) | Balanced (100 dB) | HighQuality (120 dB) |
|---|---:|---:|---:|
| 44100 → 48000 | 102.0 dB | 134.3 dB | 165.3 dB |
| 48000 → 44100 | 111.3 dB | 128.6 dB | 161.2 dB |
| 48000 → 96000 | 105.3 dB | 137.7 dB | 153.2 dB |
| 96000 → 48000 | >188 dB | >188 dB | >188 dB |
| 44100 → 22050 | >188 dB | >188 dB | >188 dB |

Every preset clears its design target with margin. The `>188 dB` entries are
**at the measurement floor**, not the filter's: the analysis window's own
sidelobes sit at −188.9 dB, so those halving conversions are cleaner than the
harness can resolve. `TestMeasurementFloor` pins that floor and fails if it
ever creeps close to the presets it is supposed to be measuring.

Other measurements, all at `HighQuality`:

```
passband flatness      ±0.0000 dB from 1% to 98% of the passband
stopband rejection     30 kHz into a 24 kHz Nyquist leaves -153.7 dBFS
DC gain error          2.9e-08 worst case
round-trip SNR         147.7 dB  (44100 -> 48000 -> 44100)
```

## The delay detail that matters

The prototype filter's length is chosen as `2·d·l + 1`, so its centre falls
on an exact multiple of `l`. The group delay is then exactly `d` input
samples for *every* polyphase phase, and is compensated internally, so output
sample `k` corresponds to input time `k·m/l` with nothing left over.

Any other length leaves a fractional residual. That sounds harmless and
isn't: a half-sample shift is a large phase error at high frequencies. An
earlier version of this package left a 0.497-sample residual, and its
round-trip SNR was **31 dB**. Fixing the prototype length took it to
**147.7 dB**, with no other change. That is the single most important thing
in this repository, and it is why the round-trip test is written as a chirp:
a periodic signal hides the problem behind its own repetition.

## Quality is requested, not selected

Give a stopband attenuation and a transition width and the filter is designed
to meet them via Kaiser's formulas:

```go
q := resample.Quality{Attenuation: 140, Transition: 0.015}
```

The presets are starting points:

| preset | attenuation | transition | taps/sample (44.1→48) |
|---|---:|---:|---:|
| `Fast` | 80 dB | 0.10 | 103 |
| `Balanced` | 100 dB | 0.05 | 259 |
| `HighQuality` | 120 dB | 0.02 | 783 |
| `Archival` | 150 dB | 0.01 | 1981 |

Cost scales as attenuation over transition width, so halving the transition
band doubles the work. `Fast` is not low quality. At 102 dB it is already
past 16-bit; it just doesn't hold the response flat as close to Nyquist.

## Performance

Per second of 44.1 kHz mono audio, on an M-series Mac:

```
Fast            4.4 ms      ~225x realtime
Balanced       12.1 ms       ~83x realtime
HighQuality    40.7 ms       ~25x realtime
```

Standard rate pairs reduce to small ratios (44100:48000 is 160/147), which
keeps both the filter table and the per-sample cost bounded. Arbitrary
ratios are approximated by continued fractions with a denominator limit.

## Scope

Mono `[]float64` in, mono `[]float64` out. Interleaved multi-channel,
fixed-point formats and file I/O are deliberately absent, so bring your own
decoder and run one `Resampler` per channel.

## Licence

BSD-3-Clause.
