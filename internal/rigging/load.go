package rigging

import "math"

func EffectiveLoad(p Point) float64 {
	return p.PlannedStaticLoadKg * p.DynamicFactor / math.Sin(p.SlingAngleDegrees*math.Pi/180)
}
