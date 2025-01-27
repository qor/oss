package s3

import (
	"errors"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	var attempts int
	maxRetries := 3
	err := Retry(maxRetries, 100*time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Test case where function never succeeds
	attempts = 0
	err = Retry(maxRetries, 100*time.Millisecond, func() error {
		attempts++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != maxRetries+1 {
		t.Errorf("Expected %d attempts, got %d", maxRetries+1, attempts)
	}
}
