package app

import "stage-rigging-release/internal/rigging"

func (a *App) Timeline(caseID string) []string { return a.Store.Timeline(caseID) }
func (a *App) OpenFindings(caseID string) []rigging.Finding {
	return rigging.OpenFindings(a.Store.Findings(caseID))
}
