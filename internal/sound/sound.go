// Package sound provides parameter-level operations on Tempest sound data.
// The Tempest sound parameter block is an unescaped ~132-byte slice; this
// package operates on those raw bytes without requiring named-parameter offsets.
package sound

// Morph linearly interpolates two parameter blocks.
// amount 0.0 returns a copy of a; 1.0 returns a copy of b.
// The shorter length is used. All output bytes are clamped to [0, 127].
func Morph(a, b []byte, amount float64) []byte {
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	n := min(len(a), len(b))
	if n == 0 {
		return nil
	}
	out := make([]byte, n)
	for i := range n {
		v := float64(a[i])*(1-amount) + float64(b[i])*amount
		if v < 0 {
			v = 0
		}
		if v > 127 {
			v = 127
		}
		out[i] = byte(v + 0.5) // round to nearest integer
	}
	return out
}
