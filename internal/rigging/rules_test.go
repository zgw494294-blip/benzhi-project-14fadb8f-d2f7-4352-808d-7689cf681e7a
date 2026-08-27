package rigging

import (
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	e := Evaluate([]Point{{ID: "p", RatedLoadKg: 100, PlannedStaticLoadKg: 20, SlingAngleDegrees: 60, DynamicFactor: 1.2}})
	if e.Outcome != "通过" || e.MinimumMarginPercent <= 0 {
		t.Fatal(e)
	}
}
func TestEquipmentExpiry(t *testing.T) {
	e := Equipment{EquipmentType: "葫芦", SerialNumber: "x", RatedLoadKg: 10, CertificateRef: "c", CertificateExpiresOn: time.Now().Add(-time.Hour), InspectionResult: "合格"}
	if ValidateEquipment(e, time.Now()) == nil {
		t.Fatal("expected expiry")
	}
}
