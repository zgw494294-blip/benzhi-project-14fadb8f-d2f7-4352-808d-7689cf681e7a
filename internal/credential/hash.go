package credential

import "crypto/sha256"

func HashText(text string) string {
	h := sha256.Sum256([]byte(text))
	const hex = "0123456789abcdef"
	out := make([]byte, 64)
	for i, b := range h {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&15]
	}
	return string(out)
}
