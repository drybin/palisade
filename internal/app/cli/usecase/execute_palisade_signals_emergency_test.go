package usecase

import (
	"testing"
	"time"
)

func TestEmergencyReason_supportBreak(t *testing.T) {
	now := time.Now().UTC()
	if got := emergencyReason(now, now, 99.6, 100, 100); got != "SUPPORT_BROKEN" {
		t.Fatalf("expected support break, got %q", got)
	}
}

func TestEmergencyReason_maxLoss(t *testing.T) {
	now := time.Now().UTC()
	if got := emergencyReason(now, now, 99.1, 90, 100); got != "MAX_LOSS" {
		t.Fatalf("expected max loss, got %q", got)
	}
}

func TestEmergencyReason_maxHoldTime(t *testing.T) {
	now := time.Now().UTC()
	if got := emergencyReason(now, now.Add(-maxPositionHold-time.Second), 100, 90, 100); got != "MAX_HOLD_TIME" {
		t.Fatalf("expected max hold time, got %q", got)
	}
}

func TestEmergencyReason_noExit(t *testing.T) {
	now := time.Now().UTC()
	if got := emergencyReason(now, now, 100, 100, 100); got != "" {
		t.Fatalf("expected no emergency exit, got %q", got)
	}
}
