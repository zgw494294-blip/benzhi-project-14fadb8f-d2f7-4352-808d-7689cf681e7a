package store

import "stage-rigging-release/internal/rigging"

func (s *Store) EvaluationOrNil(id string) any {
	e, ok := s.Evaluation(id)
	if !ok {
		return nil
	}
	return e
}
func (s *Store) Cases() []rigging.Case {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]rigging.Case, 0, len(s.cases))
	for _, c := range s.cases {
		out = append(out, c)
	}
	return out
}
