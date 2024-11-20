package utils

import "fmt"

// CustomError defines a new error type
type CustomError struct {
	Message string
	Code    uint64
}

// Error method satisfies the error interface
func (e *CustomError) Error() string {
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}
