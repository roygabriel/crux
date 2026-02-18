package docgen

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRetryer_ImmediateSuccess(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)
	calls := 0
	err := r.Do(context.Background(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryer_RetryThenSuccess(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)
	r.BaseDelay = 1 * time.Millisecond
	r.MaxDelay = 10 * time.Millisecond

	calls := 0
	err := r.Do(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryer_AllAttemptsExhausted(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)
	r.MaxAttempts = 3
	r.BaseDelay = 1 * time.Millisecond
	r.MaxDelay = 5 * time.Millisecond

	sentinel := errors.New("persistent failure")
	calls := 0
	err := r.Do(context.Background(), func() error {
		calls++
		return sentinel
	})
	if err == nil {
		t.Fatal("Do() expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("Do() error should wrap sentinel, got: %v", err)
	}
	if !strings.Contains(err.Error(), "all 3 attempts exhausted") {
		t.Errorf("Do() error = %q, want message containing 'all 3 attempts exhausted'", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryer_ContextCancellation(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)
	r.MaxAttempts = 10
	r.BaseDelay = 1 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := r.Do(ctx, func() error {
		calls++
		cancel()
		return errors.New("keep trying")
	})
	if err == nil {
		t.Fatal("Do() expected error after cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Do() error should wrap context.Canceled, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should stop after context cancelled)", calls)
	}
}

func TestRetryer_RetryableCheck(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)
	r.BaseDelay = 1 * time.Millisecond

	nonRetryable := errors.New("not retryable")
	r.RetryableCheck = func(err error) bool {
		return !errors.Is(err, nonRetryable)
	}

	calls := 0
	err := r.Do(context.Background(), func() error {
		calls++
		return nonRetryable
	})
	if err == nil {
		t.Fatal("Do() expected error, got nil")
	}
	if !errors.Is(err, nonRetryable) {
		t.Errorf("Do() error should be nonRetryable, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry non-retryable error)", calls)
	}
}

func TestRetryer_DelayGrowth(t *testing.T) {
	t.Parallel()
	r := &Retryer{
		BaseDelay: 100 * time.Millisecond,
		MaxDelay:  10 * time.Second,
		Jitter:    0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1600 * time.Millisecond},
	}

	for _, tc := range tests {
		got := r.delay(tc.attempt)
		if got != tc.want {
			t.Errorf("delay(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestRetryer_DelayMaxCap(t *testing.T) {
	t.Parallel()
	r := &Retryer{
		BaseDelay: 1 * time.Second,
		MaxDelay:  5 * time.Second,
		Jitter:    0,
	}

	got := r.delay(10) // 2^10 * 1s = 1024s, should cap at 5s
	if got != 5*time.Second {
		t.Errorf("delay(10) = %v, want %v (capped)", got, 5*time.Second)
	}
}

func TestRetryer_DelayJitter(t *testing.T) {
	t.Parallel()
	r := &Retryer{
		BaseDelay: 1 * time.Second,
		MaxDelay:  60 * time.Second,
		Jitter:    0.25,
	}

	base := float64(1 * time.Second)
	low := base * (1 - 0.25)
	high := base * (1 + 0.25)

	for range 100 {
		d := r.delay(0)
		if float64(d) < low || float64(d) > high {
			t.Errorf("delay(0) = %v, want in [%v, %v]", d, time.Duration(low), time.Duration(high))
		}
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{529, true},
	}

	for _, tc := range tests {
		got := IsRetryableHTTPStatus(tc.status)
		if got != tc.want {
			t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestDefaultRetryer_Values(t *testing.T) {
	t.Parallel()
	r := DefaultRetryer(nil)

	if r.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", r.MaxAttempts)
	}
	if r.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v, want 1s", r.BaseDelay)
	}
	if r.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, want 60s", r.MaxDelay)
	}
	if r.Jitter != 0.25 {
		t.Errorf("Jitter = %f, want 0.25", r.Jitter)
	}
	if r.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if r.RetryableCheck != nil {
		t.Error("RetryableCheck should be nil by default")
	}
}
