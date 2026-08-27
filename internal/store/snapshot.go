package store

import "stage-rigging-release/internal/credential"

func (s *Store) Evidence(id string) (credential.Evidence, error) { return s.Snapshot(id) }
