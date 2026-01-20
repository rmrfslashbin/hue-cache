package cache

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "error with key",
			err: &Error{
				Op:  "Get",
				Key: "light:123",
				Err: ErrNotFound,
			},
			want: `cache: Get "light:123": cache: key not found`,
		},
		{
			name: "error without key",
			err: &Error{
				Op:  "Clear",
				Err: ErrBackendClosed,
			},
			want: "cache: Clear: cache: backend closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlying := ErrNotFound
	err := &Error{
		Op:  "Get",
		Key: "test",
		Err: underlying,
	}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestError_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    *Error
		target error
		want   bool
	}{
		{
			name: "matches sentinel error",
			err: &Error{
				Op:  "Get",
				Key: "test",
				Err: ErrNotFound,
			},
			target: ErrNotFound,
			want:   true,
		},
		{
			name: "does not match different sentinel",
			err: &Error{
				Op:  "Get",
				Key: "test",
				Err: ErrNotFound,
			},
			target: ErrExpired,
			want:   false,
		},
		{
			name: "matches wrapped error",
			err: &Error{
				Op:  "Set",
				Key: "test",
				Err: ErrMemoryLimit,
			},
			target: ErrMemoryLimit,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	op := "Get"
	key := "light:123"
	underlying := ErrNotFound

	err := NewError(op, key, underlying)

	if err.Op != op {
		t.Errorf("Op = %v, want %v", err.Op, op)
	}
	if err.Key != key {
		t.Errorf("Key = %v, want %v", err.Key, key)
	}
	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		ErrNotFound,
		ErrExpired,
		ErrInvalidKey,
		ErrInvalidValue,
		ErrBackendClosed,
		ErrMemoryLimit,
	}

	// Check that all sentinel errors are defined and unique
	seen := make(map[string]bool)
	for _, err := range sentinels {
		if err == nil {
			t.Error("Sentinel error is nil")
			continue
		}

		msg := err.Error()
		if seen[msg] {
			t.Errorf("Duplicate sentinel error message: %s", msg)
		}
		seen[msg] = true
	}
}
