package domain

import (
	"testing"
	"time"
)

func TestTokenExpired(t *testing.T) {
	margin := 5 * time.Minute

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"zero expiry is treated as expired", time.Time{}, true},
		{"already past", time.Now().Add(-time.Hour), true},
		{"inside the safety margin", time.Now().Add(time.Minute), true},
		{"comfortably valid", time.Now().Add(time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := Token{ExpiresAt: tt.expiresAt}
			if got := token.Expired(margin); got != tt.want {
				t.Errorf("Expired(%s) = %v, want %v", margin, got, tt.want)
			}
		})
	}
}
