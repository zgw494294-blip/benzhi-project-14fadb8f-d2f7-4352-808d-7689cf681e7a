package credential

import (
	"stage-rigging-release/internal/rigging"
	"testing"
	"time"
)

func TestIssueVerify(t *testing.T) {
	e := Evidence{Case: rigging.Case{ID: "c"}}
	c := Issue(e, "r", time.Now().Add(-time.Minute), time.Now().Add(time.Minute), nil)
	if m, ok := Verify(c, e, time.Now()); !ok || m != "有效" {
		t.Fatal(m)
	}
}
