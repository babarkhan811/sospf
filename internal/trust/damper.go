package trust

// Damper smooths a noisy scalar metric (latency, load, success rate) and only
// reports a "significant" change when the smoothed value moves beyond a band
// from the last reported value. This implements the hysteresis that stops a
// noisy metric from flapping routes (brief §5.4, §5.5).
type Damper struct {
	alpha    float64 // EWMA weight on the newest sample
	band     float64 // minimum change from last reported to report again
	ewma     float64
	reported float64
	primed   bool
}

// NewDamper creates a damper. alpha in (0,1]; band is the absolute change
// required to report a new value.
func NewDamper(alpha, band float64) *Damper {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.3
	}
	return &Damper{alpha: alpha, band: band}
}

// Observe feeds a raw sample and returns (reportedValue, changed). changed is
// true only when the smoothed value has moved beyond band since the last report
// — those are the moments a new Telemetry LSA should be originated.
func (d *Damper) Observe(sample float64) (float64, bool) {
	if !d.primed {
		d.ewma = sample
		d.reported = sample
		d.primed = true
		return d.reported, true
	}
	d.ewma = d.alpha*sample + (1-d.alpha)*d.ewma
	if abs(d.ewma-d.reported) >= d.band {
		d.reported = d.ewma
		return d.reported, true
	}
	return d.reported, false
}

// Value returns the current smoothed value.
func (d *Damper) Value() float64 { return d.ewma }
