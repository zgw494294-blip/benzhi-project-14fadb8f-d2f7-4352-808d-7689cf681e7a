package nanopointvalidation

import (
	"math"
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestNaNPointRejected(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := app.New(s)
	c, err := a.CreateCase("演出", "主舞台", time.Now(), time.Now().Add(time.Hour), "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPoint(c, rigging.Point{ID: "p1", Label: "吊点1", RatedLoadKg: 100, PlannedStaticLoadKg: math.NaN(), SlingAngleDegrees: 60, DynamicFactor: 1}); err == nil {
		t.Fatal("NaN 吊点参数不应通过校验")
	}
}
