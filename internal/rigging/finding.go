package rigging

func OpenFindings(fs []Finding) []Finding {
	out := make([]Finding, 0)
	for _, f := range fs {
		if f.Status != "closed" {
			out = append(out, f)
		}
	}
	return out
}
