package shared

import (
	"log"
	"math"
	"time"
)

const EPOCH_BASE_MS = 1640995200000 // 1 Jan 2022 00:00:00 UTC
const SLOT_DURATION = 5000          // 5 sec
const PHASE_DURATION = 2000         // 2 sec
const MAX_INITIAL_CLOCK_SYNC = 1000 // 1 sec
const MAX_CLOCK_DRIFT = 100         // 100 ms
const MAX_LAG = 50                  // 50 ms

var networkTimeShift int
var latestBlockNumber int

func SetNetworkTimeShift(timeShiftMsec int) {
	alpha := 0.2   // smoothing factor
	threshold := 5 // Hysteresis correction threshold

	if timeShiftMsec < 2*threshold {
		return
	}

	if timeShiftMsec > MAX_CLOCK_DRIFT {
		log.Printf("[WARN] Time drift too large: %d\n", timeShiftMsec)
		alpha = 0.1
	}

	newShift := alpha*float64(timeShiftMsec) + (1-alpha)*float64(networkTimeShift)
	diff := int(math.Abs(float64(networkTimeShift) - newShift))
	if diff >= threshold {
		networkTimeShift = int(math.Round(newShift))
	}
}

// get (local) network time in msec
func NetworkTime() int64 {
	return time.Now().UnixMilli() + int64(networkTimeShift)
}

// get (local) network time shift in msec
func NetworkTimeShift() int {
	return networkTimeShift
}

func SlotInfo() (slotNumber int, timeLeftMsec int) {
	networkTime := NetworkTime()
	diff := networkTime - EPOCH_BASE_MS

	// assuming uniform slot duration for the entire span of the blockchain
	slotNum := diff / SLOT_DURATION
	timeLeft := (slotNum+1)*SLOT_DURATION - diff

	return int(slotNum), int(timeLeft)
}

func GetLatestBlockNumber() int {
	return latestBlockNumber
}

func SetLatestBlockNumber(newBlockNumber int) {
	if newBlockNumber > latestBlockNumber {
		latestBlockNumber = newBlockNumber
	}
}
