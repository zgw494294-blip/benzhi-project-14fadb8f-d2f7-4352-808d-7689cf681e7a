package credential

import (
	"sort"
	"stage-rigging-release/internal/rigging"
	"time"
)

type PartitionDifference struct {
	Partition, FrozenDigest, CurrentDigest string
	FirstAudit                             *rigging.AuditRecord
}

type Diagnostic struct {
	CredentialID, CaseID, Status     string
	Valid                            bool
	ValidFrom, ValidUntil, CheckedAt time.Time
	FrozenRevision, CurrentRevision  int
	Differences                      []PartitionDifference
}

func Diagnose(c Credential, current Evidence, audits []rigging.AuditRecord, now time.Time) Diagnostic {
	d := Diagnostic{CredentialID: c.ID, CaseID: c.CaseID, ValidFrom: c.ValidFrom, ValidUntil: c.ValidUntil, CheckedAt: now, FrozenRevision: c.FrozenRevision, CurrentRevision: current.Case.Revision, Differences: []PartitionDifference{}}
	status, valid := Verify(c, current, now)
	d.Status = status
	d.Valid = valid
	if status != "证据摘要不一致" {
		return d
	}
	currentParts := PartitionDigests(current)
	keys := make([]string, 0, len(c.PartitionDigests))
	for k := range c.PartitionDigests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if c.PartitionDigests[k] == currentParts[k] {
			continue
		}
		diff := PartitionDifference{Partition: k, FrozenDigest: c.PartitionDigests[k], CurrentDigest: currentParts[k]}
		for i := range audits {
			if audits[i].Seq > 0 && audits[i].Revision >= c.FrozenRevision && audits[i].Category == k {
				a := audits[i]
				diff.FirstAudit = &a
				break
			}
		}
		d.Differences = append(d.Differences, diff)
	}
	return d
}
