package solution

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestVReturnsSuccess(t *testing.T) {
	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) { return 0, errors.New("boom") },
		func(ctx context.Context) (int, error) { return 42, nil },
	}
	got, err := FirstSuccess(context.Background(), fns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d want 42", got)
	}
}

func TestVAllFailReturnsLastError(t *testing.T) {
	sentinel := errors.New("final")
	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) { return 0, errors.New("first") },
		func(ctx context.Context) (int, error) { return 0, sentinel },
	}
	_, err := FirstSuccess(context.Background(), fns)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestVEmpty(t *testing.T) {
	_, err := FirstSuccess(context.Background(), nil)
	if !errors.Is(err, ErrNoFuncs) {
		t.Fatalf("got %v want ErrNoFuncs", err)
	}
}

func TestHCancelsLosers(t *testing.T) {
	var cancelled atomic.Int64
	winner := func(ctx context.Context) (int, error) {
		return 7, nil
	}
	loser := func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			cancelled.Add(1)
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
			return 0, errors.New("timed out")
		}
	}
	fns := []func(context.Context) (int, error){loser, loser, winner, loser}
	got, err := FirstSuccess(context.Background(), fns)
	if err != nil || got != 7 {
		t.Fatalf("got (%d,%v) want (7,nil)", got, err)
	}
	// The losers should observe cancellation rather than the 2s timeout.
	deadline := time.Now().Add(time.Second)
	for cancelled.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if cancelled.Load() < 3 {
		t.Fatalf("only %d losers cancelled, want 3", cancelled.Load())
	}
}

func TestHExactlyOneWinnerAmongMany(t *testing.T) {
	const n = 200
	var winners atomic.Int64
	fns := make([]func(context.Context) (int, error), n)
	for i := range n {
		i := i
		fns[i] = func(ctx context.Context) (int, error) {
			if i == n-1 {
				winners.Add(1)
				return i, nil
			}
			return 0, errors.New("no")
		}
	}
	got, err := FirstSuccess(context.Background(), fns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != n-1 {
		t.Fatalf("got %d want %d", got, n-1)
	}
	if winners.Load() != 1 {
		t.Fatalf("winner ran %d times want 1", winners.Load())
	}
}

func TestHParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	}
	_, err := FirstSuccess(ctx, fns)
	if err == nil {
		t.Fatal("expected error from cancelled parent context")
	}
}
