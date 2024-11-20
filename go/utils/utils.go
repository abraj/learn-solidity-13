package utils

import (
	"reflect"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type IntOrString interface {
	~int | ~int64 | ~uint64 | ~string
}

type IntOrInt64 interface {
	~int | ~int64 | ~uint64
}

func Contains[T IntOrString](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func IsNil(value interface{}) bool {
	v := reflect.ValueOf(value)
	return !v.IsValid() || v.IsNil()
}

func AllNil[T any](items []T) bool {
	for _, item := range items {
		if !IsNil(item) {
			return false
		}
	}
	return true
}

// func AllNil(slice interface{}) bool {
// 	v := reflect.ValueOf(slice)
// 	if v.Kind() != reflect.Slice {
// 		panic("AllNil: provided value is not a slice")
// 	}
// 	for i := 0; i < v.Len(); i++ {
// 		if !v.Index(i).IsNil() {
// 			return false
// 		}
// 	}
// 	return true
// }

func AllEmpty(items []string) bool {
	for _, item := range items {
		if item != "" {
			return false
		}
	}
	return true
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

func MaxInt[T IntOrInt64](nums ...T) T {
	maxVal := nums[0]
	for _, num := range nums {
		if num > maxVal {
			maxVal = num
		}
	}
	return maxVal
}

func Median[T IntOrInt64](values []T) T {
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

// Map function applies a transformation function to each element in a slice
func Map[T any, U any](input []T, transform func(T) U) []U {
	result := make([]U, len(input))
	for i, v := range input {
		result[i] = transform(v)
	}
	return result
}

// Filter function applies a predicate to each element and retains those that match
func Filter[T any](input []T, predicate func(T) bool) []T {
	result := []T{}
	for _, v := range input {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce function combines elements into a single value
func Reduce[T any, U any](input []T, initial U, reducer func(U, T) U) U {
	result := initial
	for _, v := range input {
		result = reducer(result, v)
	}
	return result
}
