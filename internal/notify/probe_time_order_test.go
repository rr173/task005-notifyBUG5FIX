package notify

import (
	"testing"
	"time"
)

func TestMarkSent_RejectsTimeBeforeCreatedAt(t *testing.T) {
	s := New()
	created := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.Create(CreateInput{ID: "T1", Recipient: "u", Content: "c"}, created)

	earlier := created.Add(-1 * time.Hour)
	_, err := s.MarkSent("T1", earlier)
	if err == nil {
		t.Errorf("MarkSent should reject time before CreatedAt: sent=%v, created=%v", earlier, created)
	}
}

func TestMarkSent_AllowsTimeAfterCreatedAt(t *testing.T) {
	s := New()
	created := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.Create(CreateInput{ID: "T2", Recipient: "u", Content: "c"}, created)

	later := created.Add(1 * time.Hour)
	_, err := s.MarkSent("T2", later)
	if err != nil {
		t.Errorf("MarkSent should allow time after CreatedAt: %v", err)
	}
}
