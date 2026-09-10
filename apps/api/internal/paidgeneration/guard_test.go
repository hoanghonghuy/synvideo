package paidgeneration

import (
	"testing"
	"time"
)

func TestPolicyValidRequiresAllBounds(t *testing.T) {
	valid := Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 2, LeaseDuration: 5 * time.Minute}
	if !valid.Valid() {
		t.Fatal("expected fully bounded policy to be valid")
	}

	tests := []Policy{
		{MaxRequests: 0, Window: time.Hour, MaxInFlight: 2, LeaseDuration: 5 * time.Minute},
		{MaxRequests: 10, Window: 0, MaxInFlight: 2, LeaseDuration: 5 * time.Minute},
		{MaxRequests: 10, Window: time.Hour, MaxInFlight: 0, LeaseDuration: 5 * time.Minute},
		{MaxRequests: 10, Window: time.Hour, MaxInFlight: 2, LeaseDuration: 0},
	}
	for _, policy := range tests {
		if policy.Valid() {
			t.Fatalf("expected incomplete policy to be invalid: %+v", policy)
		}
	}
}
