package rigging

import "encoding/json"

func Encode(v any) string { b, _ := json.Marshal(v); return string(b) }
