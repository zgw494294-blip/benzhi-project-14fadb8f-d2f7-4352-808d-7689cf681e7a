package failedbatchreplaycache_test

import (
	"strings"
	"testing"
	"time"

	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
)

func TestFailedBatchAttemptIsNotReplayedAsSuccess(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a := app.New(s)
	c, err := a.CreateCase("缓存重试测试", "主舞台", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), "mechanic-a")
	if err != nil {
		t.Fatal(err)
	}
	points := []rigging.Point{{
		ID: "point-a", Label: "A1", RatedLoadKg: 500,
		PlannedStaticLoadKg: 100, SlingAngleDegrees: 60, DynamicFactor: 1.1,
	}}

	result, err := a.CommitPointBatch(c.ID, "invalid-lease", "retry-after-lease", "mechanic-a", c.Revision, points)
	if err == nil || !strings.Contains(err.Error(), "编辑租约") {
		t.Fatalf("首次提交应由 Store 因无效租约拒绝，result=%+v err=%v", result, err)
	}
	if got := s.Points(c.ID); len(got) != 0 {
		t.Fatalf("失败尝试不应写入吊点，got=%+v", got)
	}

	lease, err := a.AcquireLease(c.ID, "mechanic-a")
	if err != nil {
		t.Fatal(err)
	}
	result, err = a.CommitPointBatch(c.ID, lease.ID, "retry-after-lease", "mechanic-a", c.Revision, points)
	if err != nil {
		t.Fatalf("取得有效租约后的同 requestId 重试应成功：%v", err)
	}
	current, _ := s.GetCase(c.ID)
	if len(s.Points(c.ID)) != 1 || current.Revision != c.Revision+1 || result.AppliedRevision != c.Revision+1 {
		t.Fatalf("重试返回成功但未提交吊点和 Revision：points=%d revision=%d appliedRevision=%d", len(s.Points(c.ID)), current.Revision, result.AppliedRevision)
	}
}
