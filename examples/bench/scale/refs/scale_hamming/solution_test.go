package solution

import (
	"errors"
	"testing"
)

func TestVBasic(t *testing.T) {
	got, err := Hamming("karolin", "kathrin")
	if err != nil || got != 3 {
		t.Fatalf("got (%d,%v)", got, err)
	}
}

func TestVIdentical(t *testing.T) {
	got, err := Hamming("abc", "abc")
	if err != nil || got != 0 {
		t.Fatalf("got (%d,%v)", got, err)
	}
}

func TestHEmpty(t *testing.T) {
	got, err := Hamming("", "")
	if err != nil || got != 0 {
		t.Fatalf("empty pair: (%d,%v)", got, err)
	}
}

func TestHLengthMismatch(t *testing.T) {
	got, err := Hamming("abc", "ab")
	if !errors.Is(err, ErrLengthMismatch) {
		t.Fatalf("expected ErrLengthMismatch, got %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 on error, got %d", got)
	}
	// One empty, one not.
	if _, err := Hamming("", "x"); !errors.Is(err, ErrLengthMismatch) {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func TestHAllDiffer(t *testing.T) {
	got, err := Hamming("000", "111")
	if err != nil || got != 3 {
		t.Fatalf("got (%d,%v)", got, err)
	}
}

func TestHNonASCIIBytes(t *testing.T) {
	// Multi-byte UTF-8 compared bytewise; equal-length equal content -> 0.
	got, err := Hamming("héllo", "héllo")
	if err != nil || got != 0 {
		t.Fatalf("got (%d,%v)", got, err)
	}
}
