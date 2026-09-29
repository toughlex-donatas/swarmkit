package volumequeue

import (
	"testing"
	"time"
)

// TestRetryInterval checks that the retry interval grows exponentially up to
// maxRetryInterval and then stays there, however many attempts are made. The
// interval used to be computed as baseRetryInterval<<attempt, which overflows
// time.Duration from attempt 37 on, producing negative or zero intervals and a
// retry loop without any delay.
func TestRetryInterval(t *testing.T) {
	// 100ms<<13 is the first interval that is at least 10 minutes
	const firstCappedAttempt = 13

	if got := retryInterval(0); got != 0 {
		t.Errorf("attempt 0: expected no wait, got %v", got)
	}

	var prev time.Duration
	for attempt := uint(0); attempt <= 200; attempt++ {
		got := retryInterval(attempt)
		if got < 0 || got > maxRetryInterval {
			t.Errorf("attempt %d: interval %v is outside [0, %v]", attempt, got, maxRetryInterval)
		}
		if got < prev {
			t.Errorf("attempt %d: interval %v is less than the previous interval %v", attempt, got, prev)
		}
		if attempt > 0 && attempt < firstCappedAttempt {
			if want := baseRetryInterval << attempt; got != want {
				t.Errorf("attempt %d: expected interval %v, got %v", attempt, want, got)
			}
		}
		if attempt >= firstCappedAttempt && got != maxRetryInterval {
			t.Errorf("attempt %d: expected interval %v, got %v", attempt, maxRetryInterval, got)
		}
		prev = got
	}
}
