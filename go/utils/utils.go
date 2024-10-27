package utils

import (
	"sort"
	"time"
)

func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
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
