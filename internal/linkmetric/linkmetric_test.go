package linkmetric

import "testing"

func TestCongestionRaisesCost(t *testing.T) {
	m := New(10, DefaultWeights())
	// A healthy link stays near base.
	if c, _ := m.Observe(1, 0.0); c > 12 {
		t.Errorf("healthy link cost should stay near base, got %d", c)
	}
	// Sustained congestion (high latency + load) should raise it well above base.
	var c uint16
	for i := 0; i < 30; i++ {
		c, _ = m.Observe(200, 0.9)
	}
	if c <= 12 {
		t.Errorf("congested link cost should rise above base, got %d", c)
	}
}

func TestNoFlapWithinBand(t *testing.T) {
	w := DefaultWeights()
	w.Band = 3
	m := New(20, w)
	m.Observe(10, 0.1) // settle
	base := m.Reported()
	changes := 0
	// Small noise around the same operating point must not re-advertise.
	for _, lat := range []float64{11, 9, 12, 8, 10, 11, 9} {
		if _, changed := m.Observe(lat, 0.1); changed {
			changes++
		}
	}
	if changes != 0 {
		t.Errorf("noise within the band should not re-advertise; got %d changes (base=%d, now=%d)", changes, base, m.Reported())
	}
}

func TestSustainedShiftDoesReport(t *testing.T) {
	m := New(10, DefaultWeights())
	m.Observe(1, 0.0)
	reported := false
	for i := 0; i < 40; i++ {
		if _, changed := m.Observe(150, 0.5); changed {
			reported = true
			break
		}
	}
	if !reported {
		t.Error("a sustained congestion shift should eventually re-advertise")
	}
}
