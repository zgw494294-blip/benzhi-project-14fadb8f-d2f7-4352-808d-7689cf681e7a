package rigging

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func NormalizeAndValidatePoints(existing, submitted []Point, caseID string, baseRevision int) BatchPointResult {
	result := BatchPointResult{CaseID: caseID, BaseRevision: baseRevision, ExpectedRevision: baseRevision + 1, Changes: []PointChange{}, Problems: []RowProblem{}}
	byID := make(map[string]Point, len(existing))
	labels := make(map[string]string, len(existing)+len(submitted))
	for _, p := range existing {
		byID[p.ID] = p
		labels[strings.ToLower(strings.TrimSpace(p.Label))] = p.ID
	}
	seenID := map[string]int{}
	seenLabel := map[string]int{}
	for row, raw := range submitted {
		p := raw
		p.CaseID = caseID
		p.ID = strings.TrimSpace(p.ID)
		p.Label = strings.TrimSpace(p.Label)
		p.RedundancyGroup = strings.TrimSpace(p.RedundancyGroup)
		if p.ID == "" {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "id", "required", "吊点标识不能为空"))
		}
		if prior, ok := seenID[p.ID]; ok && p.ID != "" {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "id", "duplicate_id", fmt.Sprintf("吊点标识与第%d行重复", prior+1)))
		} else {
			seenID[p.ID] = row
		}
		labelKey := strings.ToLower(p.Label)
		if p.Label == "" {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "label", "required", "吊点标签不能为空"))
		}
		if prior, ok := seenLabel[labelKey]; ok && labelKey != "" {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "label", "duplicate_label", fmt.Sprintf("吊点标签与第%d行重复", prior+1)))
		} else {
			seenLabel[labelKey] = row
		}
		if owner, ok := labels[labelKey]; ok && owner != p.ID {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "label", "duplicate_label", "吊点标签已被其他吊点使用"))
		}
		if math.IsNaN(p.XMillimeters) || math.IsInf(p.XMillimeters, 0) || math.Abs(p.XMillimeters) > 100000 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "xMillimeters", "invalid_coordinate", "X坐标必须是绝对值不超过100000毫米的有限数"))
		}
		if math.IsNaN(p.YMillimeters) || math.IsInf(p.YMillimeters, 0) || math.Abs(p.YMillimeters) > 100000 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "yMillimeters", "invalid_coordinate", "Y坐标必须是绝对值不超过100000毫米的有限数"))
		}
		if p.RatedLoadKg <= 0 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "ratedLoadKg", "invalid_capacity", "额定承载必须大于0"))
		}
		if p.PlannedStaticLoadKg <= 0 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "plannedStaticLoadKg", "invalid_static_load", "预定静载必须大于0"))
		}
		if p.SlingAngleDegrees <= 0 || p.SlingAngleDegrees >= 90 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "slingAngleDegrees", "invalid_angle", "吊索角度必须大于0且小于90度"))
		}
		if p.DynamicFactor < 1 || p.DynamicFactor > 5 {
			result.Problems = append(result.Problems, pointProblem(row, p.ID, "dynamicFactor", "invalid_dynamic_factor", "动态系数必须在1到5之间"))
		}
		before, exists := byID[p.ID]
		change := PointChange{Row: row, Action: "新增", After: p}
		if exists {
			b := before
			change.Before = &b
			change.Action = "更新"
		}
		result.Changes = append(result.Changes, change)
	}
	sort.Slice(result.Problems, func(i, j int) bool {
		if result.Problems[i].Row == result.Problems[j].Row {
			return result.Problems[i].Field < result.Problems[j].Field
		}
		return result.Problems[i].Row < result.Problems[j].Row
	})
	result.Valid = len(result.Problems) == 0
	return result
}

func pointProblem(row int, id, field, code, message string) RowProblem {
	return RowProblem{Row: row, PointID: id, Field: field, Code: code, Message: message}
}
