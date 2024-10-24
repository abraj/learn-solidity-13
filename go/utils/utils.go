package utils

import "time"

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
