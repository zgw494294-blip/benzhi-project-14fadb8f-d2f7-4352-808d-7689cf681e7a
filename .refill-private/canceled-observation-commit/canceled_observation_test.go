package canceled_observation_commit_test

import (
	"context"
	"io"
	"net/http/httptest"
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/httpui"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"strings"
	"sync"
	"testing"
	"time"
)

type gatedBody struct {
	payload []byte
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *gatedBody) Read(p []byte) (int, error) {
	b.once.Do(func() {
		close(b.started)
		<-b.release
	})
	if len(b.payload) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.payload)
	b.payload = b.payload[n:]
	return n, nil
}

func TestCanceledObservationDoesNotCommit(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/rigging.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	caseID := "case-context-cancel"
	now := time.Now()
	st.PutCase(rigging.Case{
		ID: caseID, ShowName: "取消测试演出", VenueZone: "主舞台",
		PerformanceStartsAt: now, PerformanceEndsAt: now.Add(time.Hour),
		Status: rigging.StatusEvaluated, Revision: 2, AuthorID: "mechanic",
		CreatedAt: now, UpdatedAt: now,
	})
	st.AddPoint(rigging.Point{
		ID: "point-1", CaseID: caseID, Label: "主吊点",
		RatedLoadKg: 1000, PlannedStaticLoadKg: 100,
		SlingAngleDegrees: 60, DynamicFactor: 1.2,
	})
	st.PutEvaluation(rigging.Evaluation{
		ID: "evaluation-1", CaseID: caseID, CaseRevision: 2,
		Outcome: "通过", EvaluatedAt: now,
	})

	body := &gatedBody{
		payload: []byte(`{"actor":"mechanic","pointId":"point-1","type":"位移","description":"正常","measurements":[{"value":0.2}]}`),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/cases/"+caseID+"/observations", body).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		httpui.New(app.New(st)).Handler().ServeHTTP(recorder, req)
		close(done)
	}()

	<-body.started
	cancel()
	close(body.release)
	<-done

	if got := len(st.Observations(caseID)); got != 0 {
		t.Fatalf("已取消的请求仍提交了 %d 条观察，HTTP 状态为 %d", got, recorder.Code)
	}
	if recorder.Code < 400 || !strings.Contains(recorder.Body.String(), "取消") {
		t.Fatalf("已取消的请求应返回取消错误，实际状态为 %d，响应为 %s", recorder.Code, recorder.Body.String())
	}
}
