package shared

import (
	"log"
	"math"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
)

const EPOCH_BASE_MS uint64 = 1640995200000 // 1 Jan 2022 00:00:00 UTC
const SLOT_DURATION uint64 = 5000          // 5 sec
const PHASE_DURATION uint64 = 2000         // 2 sec
const MAX_INITIAL_CLOCK_SYNC uint64 = 1000 // 1 sec
const MAX_CLOCK_DRIFT uint64 = 100         // 100 ms
const MAX_LAG uint64 = 50                  // 50 ms

var networkTimeShift int64
var latestBlockNumber int64 = -1
var privateKey crypto.PrivKey

func SetNetworkTimeShift(timeShiftMsec int64) {
	var alpha float64 = 0.2    // smoothing factor
	const threshold uint64 = 5 // Hysteresis correction threshold

	if uint64(math.Abs(float64(timeShiftMsec))) < 2*threshold {
		return
	}

	if timeShiftMsec > int64(MAX_CLOCK_DRIFT) {
		log.Printf("[WARN] Time drift too large: %d\n", timeShiftMsec)
		alpha = 0.1
	}

	newShift := alpha*float64(timeShiftMsec) + (1-alpha)*float64(networkTimeShift)
	diff := uint64(math.Abs(float64(networkTimeShift) - newShift))
	if diff >= threshold {
		networkTimeShift = int64(math.Round(newShift))
	}
}

// get (local) network time in msec
func NetworkTime() uint64 {
	return uint64(time.Now().UnixMilli() + int64(networkTimeShift))
}

// get (local) network time shift in msec
func NetworkTimeShift() int64 {
	return networkTimeShift
}

func SlotInfo() (slotNumber uint64, timeLeftMsec uint64) {
	networkTime := NetworkTime()
	diff := networkTime - EPOCH_BASE_MS

	// assuming uniform slot duration for the entire span of the blockchain
	slotNum := diff / SLOT_DURATION
	timeLeft := (slotNum+1)*SLOT_DURATION - diff

	return slotNum, timeLeft
}

func GetLatestBlockNumber() uint64 {
	if latestBlockNumber < 0 {
		log.Fatalf("[PANIC] latestBlockNumber not yet set: %d\n", latestBlockNumber)
	}
	return uint64(latestBlockNumber)
}

func SetLatestBlockNumber(newBlockNumber uint64) {
	if int64(newBlockNumber) > latestBlockNumber {
		latestBlockNumber = int64(newBlockNumber)
	}
}

func NextBlockInfo(initialCall bool) (uint64, uint64) {
	var nextBlockNumber uint64
	var waitTimeMsec uint64

	slotNumber, timeLeftMsec := SlotInfo()
	timeElapsed := SLOT_DURATION - timeLeftMsec
	if timeElapsed < MAX_LAG {
		// submit for this block, you're already late!
		nextBlockNumber = slotNumber
		waitTimeMsec = 0
	} else {
		// schedule for next block
		nextBlockNumber = slotNumber + 1
		waitTimeMsec = timeLeftMsec
	}

	if !initialCall {
		if nextBlockNumber <= GetLatestBlockNumber() {
			log.Printf("[WARN] Skipping slot %d ..\n", nextBlockNumber)
			return 0, timeLeftMsec + SLOT_DURATION
		}
	}

	return nextBlockNumber, waitTimeMsec
}

func SetPrivateKey(privKey crypto.PrivKey) {
	privateKey = privKey
}

func GetPrivateKey() crypto.PrivKey {
	return privateKey
}
