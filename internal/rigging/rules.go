package rigging

import "time"

func FindingFromObservation(caseID, pointID, typ, desc string, value float64, pass bool) Finding {
	f := Finding{ID: caseID + "-" + pointID + "-" + time.Now().Format("150405.000"), CaseID: caseID, PointID: pointID, ObservationType: typ, Description: desc, MeasuredValue: value, OpenedAt: time.Now(), Status: "closed"}
	if !pass {
		f.Status = "open"
		f.Severity = "high"
	}
	return f
}
func RemediationScope(f Finding, action string, points []Point, equipment []Equipment) Remediation {
	r := Remediation{FindingID: f.ID, CaseID: f.CaseID, ActionType: action, SubmittedAt: time.Now(), AffectedPointIDs: []string{f.PointID}, BeforeSnapshot: f.Description}
	for _, p := range points {
		if p.ID == f.PointID {
			r.AfterSnapshot = "调整吊点 " + p.Label
		}
	}
	if action == "器材替换" {
		for _, e := range equipment {
			r.AffectedEquipmentIDs = append(r.AffectedEquipmentIDs, e.ID)
		}
	}
	return r
}
