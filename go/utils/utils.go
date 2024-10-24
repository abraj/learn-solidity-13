package utils

import (
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func IsValidator(peerID peer.ID, validators []peer.ID) bool {
	includes := false
	for _, v := range validators {
		if v.String() == peerID.String() {
			includes = true
		}
	}
	return includes
}

func SetInterval(fn func(), interval time.Duration) chan struct{} {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fn() // Call the provided function at each tick
			case <-done:
				return // Exit the Goroutine when done is closed
			}
		}
	}()

	return done // Return the channel to stop the interval
}

func ExpBackOff(fn func(), initialInterval time.Duration, finalInterval time.Duration) chan struct{} {
	currentInterval := initialInterval
	backoffActive := true

	ticker := time.NewTicker(currentInterval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if backoffActive {
					currentInterval = 2 * currentInterval
					if currentInterval < finalInterval {
						ticker.Reset(currentInterval)
					} else {
						ticker.Reset(finalInterval)
						backoffActive = false
					}
				}
				fn() // Call the provided function at each tick
			case <-done:
				return // Exit the Goroutine when done is closed
			}
		}
	}()

	return done // Return the channel to stop the interval
}

func Median(values []int64) int64 {
	n := len(values)
	if n == 0 {
		return 0 // Return 0 or an error for an empty slice
	}

	// Sort the slice
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	// Calculate the median
	if n%2 == 1 {
		// Odd length: return the middle element
		// return float64(values[n/2])
		return values[n/2]
	} else {
		// Even length: return the average of the two middle elements
		mid1 := values[(n/2)-1]
		mid2 := values[n/2]
		// return float64(mid1+mid2) / 2
		return (mid1 + mid2) / 2 // integer division
	}
}
