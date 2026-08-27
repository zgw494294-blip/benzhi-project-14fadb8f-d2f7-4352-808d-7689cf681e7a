package app

import "stage-rigging-release/internal/rigging"

type Summary struct {
	Case                                         rigging.Case
	EquipmentCount, PointCount, OpenFindingCount int
	Outcome                                      string
}

func (a *App) Summary(id string) Summary {
	c, _ := a.Store.GetCase(id)
	ev, _ := a.Store.Evaluation(id)
	return Summary{Case: c, EquipmentCount: len(a.Store.Equipment(id)), PointCount: len(a.Store.Points(id)), OpenFindingCount: len(a.OpenFindings(id)), Outcome: ev.Outcome}
}
