package utils

// 800ms => 10000 steps (100*100)
func MsecToSteps[T IntOrInt64](durationMsec T) int64 {
	units := int64(durationMsec) / 8
	steps := units * 100
	return steps
}
