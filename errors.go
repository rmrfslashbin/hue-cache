package cache

import (
	"errors"
	"fmt"
)

// Sentinel errors for cache operations.
var (
	// ErrNotFound is returned when a cache key is not found.
	ErrNotFound = errors.New("cache: key not found")

	// ErrExpired is returned when a cache entry has expired.
	ErrExpired = errors.New("cache: entry expired")

	// ErrInvalidKey is returned when a cache key is invalid (empty or malformed).
	ErrInvalidKey = errors.New("cache: invalid key")

	// ErrInvalidValue is returned when a cache value is invalid (nil or too large).
	ErrInvalidValue = errors.New("cache: invalid value")

	// ErrBackendClosed is returned when operating on a closed backend.
	ErrBackendClosed = errors.New("cache: backend closed")

	// ErrMemoryLimit is returned when an operation would exceed memory limits.
	ErrMemoryLimit = errors.New("cache: memory limit exceeded")
)

// Error wraps cache errors with additional context.
type Error struct {
	// Op is the operation that failed (e.g., "Get", "Set").
	Op string

	// Key is the cache key involved in the error.
	Key string

	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("cache: %s %q: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("cache: %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is implements error comparison for sentinel errors.
func (e *Error) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewError creates a new cache error.
func NewError(op, key string, err error) *Error {
	return &Error{
		Op:  op,
		Key: key,
		Err: err,
	}
}
