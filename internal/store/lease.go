package store

import (
	"stage-rigging-release/internal/rigging"
	"time"
)

func (s *Store) AcquireLease(c rigging.Case, holder string) (rigging.Lease, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.leasing[c.ID]; ok && old.Active(time.Now()) && old.Holder != holder {
		return rigging.Lease{}, false
	}
	l := rigging.Lease{ID: c.ID + "-lease-" + holder, CaseID: c.ID, Holder: holder, ExpiresAt: time.Now().Add(15 * time.Minute)}
	c.EditLeaseID = l.ID
	s.cases[c.ID] = c
	s.leasing[c.ID] = l
	s.auditLocked(c.ID, "case", c.ID, "", holder, "取得草稿编辑租约", c.Revision)
	_ = s.persistLocked()
	return l, true
}

func (s *Store) Lease(caseID string) (rigging.Lease, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.leasing[caseID]
	return l, ok
}
