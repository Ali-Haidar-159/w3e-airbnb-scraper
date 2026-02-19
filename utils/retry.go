package utils

import (
	"fmt"
	"time"
)

// RetryWithBackoff retries a function up to maxRetries times with exponential backoff
func RetryWithBackoff(maxRetries int, fn func() error, logger *Logger) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			logger.Warn("Retrying (attempt %d/%d) after %v...", attempt+1, maxRetries, backoff)
			time.Sleep(backoff)
		}
		if err := fn(); err != nil {
			lastErr = err
			logger.Error("Attempt %d failed: %v", attempt+1, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("all %d attempts failed, last error: %w", maxRetries, lastErr)
}