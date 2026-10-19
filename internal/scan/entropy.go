package scan

import "math"

// shannonEntropy returns the bits-per-character entropy of s. A random API key
// scores high (~4+ over base64); English prose and repeated characters score
// low. It is the signal behind the generic high-entropy detector (plan.md §4.1
// step 5) for secrets no explicit rule matches.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}

	n := float64(len(s))
	var e float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		e -= p * math.Log2(p)
	}
	return e
}
