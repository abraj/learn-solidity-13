package shared

import (
	"math"
	"time"
)

var networkTimeShift int

func SetNetworkTimeShift(adjustedShift int) {
	alpha := 0.2   // smoothing factor
	threshold := 5 // Hysteresis correction threshold

	if adjustedShift < 2*threshold {
		return
	}

	newShift := alpha*float64(adjustedShift) + (1-alpha)*float64(networkTimeShift)
	diff := int(math.Abs(float64(networkTimeShift) - newShift))
	if diff >= threshold {
		networkTimeShift = int(math.Round(newShift))
	}
}

func NetworkTime() int64 {
	return time.Now().UnixMilli() + int64(networkTimeShift)
}
