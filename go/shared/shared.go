package shared

import (
	"log"
	"math"
	"time"
)

const EPOCH_BASE_MS = 1640995200000 // 1 January 2022 00:00:00 UTC
const SLOT_DURATION = 5000          // 5 sec
const BUFFER_TIME = 50              // 50 ms

var networkTimeShift int
var latestBlockNumber int

func SetNetworkTimeShift(adjustedShift int) {
	alpha := 0.2   // smoothing factor
	threshold := 5 // Hysteresis correction threshold

	if adjustedShift < 2*threshold {
		return
	} else if adjustedShift > SLOT_DURATION/2 {
		log.Fatalf("[ERROR] Time drift too large! Won't perform network time sync.\n")
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

func SlotInfo() (slotNumber int, timeLeftMsec int) {
	networkTime := NetworkTime()
	diff := networkTime - EPOCH_BASE_MS
	slot := diff / SLOT_DURATION
	timeLeft := (slot+1)*SLOT_DURATION - diff
	return int(slot), int(timeLeft)
}

func GetLatestBlockNumber() int {
	return latestBlockNumber
}

func SetLatestBlockNumber(newBlockNumber int) {
	if newBlockNumber > latestBlockNumber {
		latestBlockNumber = newBlockNumber
	}
}
