package store

import (
	"stage-rigging-release/internal/rigging"
	"testing"
)

func TestStore(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c := rigging.Case{ID: "c"}
	s.PutCase(c)
	if _, ok := s.GetCase("c"); !ok {
		t.Fatal("missing")
	}
}
