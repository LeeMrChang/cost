package domain

import (
	"testing"
	"time"
)

func TestMonthBounds(t *testing.T) {
	start, end, err := Bounds("2024-02")
	if err != nil {
		t.Fatal(err)
	}
	if end.Sub(start) != 29*24*time.Hour {
		t.Fatal("leap month boundary")
	}
	if start.UTC().Format(time.RFC3339) != "2024-01-31T16:00:00Z" {
		t.Fatal("timezone boundary")
	}
	if _, _, err = Bounds("2024-13"); err == nil {
		t.Fatal("invalid month accepted")
	}
}
func TestSummary(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	entries := []Entry{{CategoryID: 1, Kind: "expense", Amount: 700, OccurredAt: at}, {CategoryID: 2, Kind: "expense", Amount: 1800, OccurredAt: at}, {CategoryID: 9, Kind: "income", Amount: 5000, OccurredAt: at}}
	s := Summarize(entries, nil)
	if s.Expense != 2500 || s.Income != 5000 || s.Balance != 2500 || s.Remaining != nil || len(s.Categories) != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	b := int64(2000)
	s = Summarize(entries, &b)
	if *s.Remaining != -500 {
		t.Fatal("overspending")
	}
}
