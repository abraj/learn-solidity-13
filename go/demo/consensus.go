package demo

import (
	"fmt"
	"libp2pdemo/shared"
	"math/rand"
	"time"
)

func nextBlockInfo() (int, int) {
	var nextBlockNumber int
	var waitTimeMsec int

	slotNumber, timeLeftMsec := shared.SlotInfo()
	if timeLeftMsec > (shared.SLOT_DURATION - shared.BUFFER_TIME) {
		// submit for this block
		nextBlockNumber = slotNumber
		waitTimeMsec = 0
	} else {
		// schedule for next block
		nextBlockNumber = slotNumber + 1
		waitTimeMsec = timeLeftMsec
	}

	if nextBlockNumber <= shared.GetLatestBlockNumber() {
		return 0, shared.SLOT_DURATION / 2
	}
	return nextBlockNumber, waitTimeMsec
}

func triggerPhase1(nextBlockNumber int) {
	go func() {
		fmt.Println(">> nextBlockNumber:", nextBlockNumber)
	}()
}

func InitConsensusLoop() {
	go func() {
		time.Sleep(10 * time.Second)

		for {
			nextBlockNumber, waitTimeMsec := nextBlockInfo()
			time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			if nextBlockNumber > 0 {
				shared.SetLatestBlockNumber(nextBlockNumber)
				triggerPhase1(nextBlockNumber)
			}
			duration := shared.BUFFER_TIME + int(rand.Float64()*1000)
			time.Sleep(time.Duration(duration) * time.Millisecond)
		}
	}()
}
