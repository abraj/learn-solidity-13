package shared

import (
	"math"
	"time"
)

const EPOCH_BASE_MS = 1640995200000 // 1 January 2022 00:00:00 UTC
const SLOT_DURATION = 5000          // 5 sec

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

func SlotInfo() (int, float32) {
	networkTime := NetworkTime()
	diff := networkTime - EPOCH_BASE_MS
	slot := diff / SLOT_DURATION
	timeLeft := (slot+1)*SLOT_DURATION - diff
	timeLeftSec := math.Round(float64(timeLeft)/100) / 10
	return int(slot), float32(timeLeftSec)
}
