package persistence_error_observation_test

import (
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"strings"
	"testing"
)

func TestObservationPersistenceErrorPropagated(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/rigging.db")
	if err != nil {
		t.Fatal(err)
	}

	const caseID = "case-persistence-failure"
	s.PutCase(rigging.Case{ID: caseID, Status: rigging.StatusEvaluated, Revision: 3})
	s.AddPoint(rigging.Point{ID: "point-1", CaseID: caseID, Label: "P1"})
	s.PutEvaluation(rigging.Evaluation{ID: "evaluation-1", CaseID: caseID, Outcome: "通过"})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = app.New(s).RecordObservation(caseID, "mechanic-1", rigging.Observation{
		PointID:      "point-1",
		Type:         "位移",
		Measurements: []rigging.Measurement{{Value: 1}},
	})
	if err == nil {
		t.Fatalf("TestObservationPersistenceErrorPropagated: 持久化连接失效后仍返回成功")
	}
	if !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("TestObservationPersistenceErrorPropagated: 错误链未保留底层原因: %v", err)
	}
}
