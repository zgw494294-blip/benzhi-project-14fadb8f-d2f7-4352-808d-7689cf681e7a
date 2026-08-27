package app

import (
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestFlow(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	a := New(s)
	c, e := a.CreateCase("演出", "主舞台", time.Now(), time.Now().Add(time.Hour), "a")
	if e != nil {
		t.Fatal(e)
	}
	p := rigging.Point{ID: "p", CaseID: c.ID, Label: "P1", RatedLoadKg: 100, PlannedStaticLoadKg: 10, SlingAngleDegrees: 60, DynamicFactor: 1.1}
	s.AddPoint(p)
	for i, typ := range []string{"葫芦", "钢索", "卸扣", "安全绳"} {
		id := string(rune('a' + i))
		s.AddEquipment(rigging.Equipment{ID: id, CaseID: c.ID, EquipmentType: typ, SerialNumber: "s-" + id, RatedLoadKg: 100, CertificateRef: "cert", CertificateExpiresOn: time.Now().Add(time.Hour), InspectionResult: "合格"})
		s.AddEquipment(rigging.Equipment{ID: id + "r", CaseID: c.ID, EquipmentType: typ, SerialNumber: "sr-" + id, RatedLoadKg: 100, CertificateRef: "cert", CertificateExpiresOn: time.Now().Add(time.Hour), InspectionResult: "合格"})
	}
	assignments := []rigging.Assignment{}
	for i := range 4 {
		id := string(rune('a' + i))
		assignments = append(assignments, rigging.Assignment{CaseID: c.ID, PointID: p.ID, Path: "主路径", EquipmentID: id}, rigging.Assignment{CaseID: c.ID, PointID: p.ID, Path: "冗余路径", EquipmentID: id + "r"})
	}
	if _, e = a.SaveAssignments(c.ID, "a", c.Revision, assignments); e != nil {
		t.Fatal(e)
	}
	c, _ = s.GetCase(c.ID)
	if _, e = a.Evaluate(c); e != nil {
		t.Fatal(e)
	}
}
