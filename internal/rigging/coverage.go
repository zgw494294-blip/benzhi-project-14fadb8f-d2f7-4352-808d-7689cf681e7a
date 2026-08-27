package rigging

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

var requiredEquipmentTypes = []string{"葫芦", "钢索", "卸扣", "安全绳"}

func BuildCoverage(c Case, points []Point, equipment []Equipment, assignments []Assignment) CoverageMatrix {
	m := CoverageMatrix{CaseID: c.ID, Status: "完整覆盖", Rows: []PathCoverage{}, Problems: []CoverageProblem{}}
	pointByID := map[string]Point{}
	equipmentByID := map[string]Equipment{}
	occupied := map[string]Assignment{}
	for _, p := range points {
		pointByID[p.ID] = p
	}
	for _, e := range equipment {
		equipmentByID[e.ID] = e
	}
	groups := map[string][]Assignment{}
	for _, a := range assignments {
		if a.Path != "主路径" && a.Path != "冗余路径" {
			m.Problems = append(m.Problems, covProblem(a, "invalid_path", "配装路径必须是主路径或冗余路径"))
			continue
		}
		if _, ok := pointByID[a.PointID]; !ok {
			m.Problems = append(m.Problems, covProblem(a, "point_not_found", "吊点不存在"))
			continue
		}
		if _, ok := equipmentByID[a.EquipmentID]; !ok {
			m.Problems = append(m.Problems, covProblem(a, "equipment_not_found", "器材不存在"))
			continue
		}
		serialKey := strings.ToLower(strings.TrimSpace(equipmentByID[a.EquipmentID].SerialNumber))
		if prior, ok := occupied[serialKey]; ok {
			m.Problems = append(m.Problems, covProblem(a, "exclusive_occupation", fmt.Sprintf("器材已用于%s的%s", prior.PointID, prior.Path)))
			continue
		}
		occupied[serialKey] = a
		groups[a.PointID+"\x00"+a.Path] = append(groups[a.PointID+"\x00"+a.Path], a)
	}
	for _, p := range points {
		for _, path := range []string{"主路径", "冗余路径"} {
			as := groups[p.ID+"\x00"+path]
			row := PathCoverage{PointID: p.ID, Path: path, RequiredLoadKg: EffectiveLoad(p), CapacityKg: math.MaxFloat64, CertificateValidUntil: time.Time{}, Complete: true}
			types := map[string]bool{}
			for _, a := range as {
				e := equipmentByID[a.EquipmentID]
				types[e.EquipmentType] = true
				row.EquipmentTypes = append(row.EquipmentTypes, e.EquipmentType)
				if e.RatedLoadKg < row.CapacityKg {
					row.CapacityKg = e.RatedLoadKg
					row.ShortfallEquipmentID = e.ID
				}
				if row.CertificateValidUntil.IsZero() || e.CertificateExpiresOn.Before(row.CertificateValidUntil) {
					row.CertificateValidUntil = e.CertificateExpiresOn
				}
				if e.CertificateRef == "" {
					row.Complete = false
					m.Problems = append(m.Problems, covProblem(a, "certificate_missing", "器材缺少合格证"))
				}
				if e.CertificateExpiresOn.Before(c.PerformanceStartsAt) {
					row.Complete = false
					m.Problems = append(m.Problems, covProblem(a, "certificate_expires_before_show", "器材合格证在演出开始前到期"))
				}
				if e.InspectionResult != "合格" {
					row.Complete = false
					m.Problems = append(m.Problems, covProblem(a, "inspection_failed", "器材现场检查不合格"))
				}
			}
			if len(as) == 0 {
				row.CapacityKg = 0
			}
			for _, typ := range requiredEquipmentTypes {
				if !types[typ] {
					row.MissingTypes = append(row.MissingTypes, typ)
				}
			}
			if len(row.MissingTypes) > 0 {
				row.Complete = false
				m.Problems = append(m.Problems, CoverageProblem{PointID: p.ID, Path: path, Code: "equipment_type_missing", Message: "缺少器材类型：" + strings.Join(row.MissingTypes, "、")})
			}
			row.MarginKg = row.CapacityKg - row.RequiredLoadKg
			if row.MarginKg < 0 {
				row.Complete = false
				m.Problems = append(m.Problems, CoverageProblem{PointID: p.ID, Path: path, EquipmentID: row.ShortfallEquipmentID, Code: "chain_capacity_insufficient", Message: "链路短板器材能力不足"})
			}
			if !row.Complete {
				m.Status = "阻断"
			}
			sort.Strings(row.EquipmentTypes)
			m.Rows = append(m.Rows, row)
		}
	}
	sort.Slice(m.Rows, func(i, j int) bool {
		if m.Rows[i].PointID == m.Rows[j].PointID {
			return m.Rows[i].Path < m.Rows[j].Path
		}
		return m.Rows[i].PointID < m.Rows[j].PointID
	})
	if len(m.Problems) > 0 {
		m.Status = "阻断"
	}
	return m
}

func covProblem(a Assignment, code, message string) CoverageProblem {
	return CoverageProblem{PointID: a.PointID, Path: a.Path, EquipmentID: a.EquipmentID, Code: code, Message: message}
}

func EvaluateWithCoverage(points []Point, matrix CoverageMatrix) Evaluation {
	e := Evaluate(points)
	capacity := map[string]float64{}
	for _, row := range matrix.Rows {
		if prior, ok := capacity[row.PointID]; !ok || row.CapacityKg < prior {
			capacity[row.PointID] = row.CapacityKg
		}
	}
	e.Outcome = "通过"
	e.OverLimitPointIDs = nil
	e.MinimumMarginPercent = math.MaxFloat64
	for i := range e.PointResults {
		r := &e.PointResults[i]
		if c, ok := capacity[r.PointID]; ok && c < r.CapacityKg {
			r.CapacityKg = c
		}
		if r.CapacityKg <= 0 {
			r.MarginPercent = -100
		} else {
			r.MarginPercent = (r.CapacityKg - r.EffectiveLoadKg) / r.CapacityKg * 100
		}
		r.OverLimit = r.MarginPercent < 0
		if r.MarginPercent < e.MinimumMarginPercent {
			e.MinimumMarginPercent = r.MarginPercent
		}
		if r.OverLimit {
			e.Outcome = "阻断"
			e.OverLimitPointIDs = append(e.OverLimitPointIDs, r.PointID)
		}
	}
	if len(points) == 0 {
		e.MinimumMarginPercent = 0
		e.Outcome = "阻断"
	}
	if matrix.Status != "完整覆盖" {
		e.Outcome = "阻断"
	}
	return e
}
