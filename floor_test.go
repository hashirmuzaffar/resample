package resample

import "testing"

// TestMeasurementFloor establishes what the test harness itself can resolve.
// A pure tone that has been through no resampler at all still shows a finite
// SFDR, set by double-precision round-off and the analysis window's own
// sidelobes. Any measured figure at or near this floor is a property of the
// measurement, not of the resampler.
func TestMeasurementFloor(t *testing.T) {
	const n = 1 << 15
	for _, beta := range []float64{12, 16, 20, 24} {
		x := tone(n, 0.30*22050, 44100)
		got, at := sfdr(spectrumDB(x, beta), 8)
		t.Logf("unresampled tone, Kaiser beta %4.0f: SFDR %7.2f dB (bin %d)", beta, got, at)
	}

	// The floor at the beta the quality tests use must leave headroom
	// above the most demanding preset, or those tests would be measuring
	// this window rather than the resampler.
	floor, _ := sfdr(spectrumDB(tone(n, 0.30*22050, 44100), analysisBeta), 8)
	t.Logf("floor at the analysis beta of %.0f: %.2f dB", analysisBeta, floor)
	if floor < Archival.Attenuation+20 {
		t.Errorf("measurement floor %.2f dB leaves too little headroom above the %.0f dB preset",
			floor, Archival.Attenuation)
	}
}
