package utils

// 800ms => 10000 steps (100*100)
func MsecToSteps(durationMsec uint64) uint64 {
	units := uint64(durationMsec) / 8
	steps := units * 100
	return steps
}
