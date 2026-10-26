package domain

import (
	"errors"
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, cst())
}

func TestRangeBoundaries(t *testing.T) {
	cases := []struct {
		name, kind, ref, start, end   string
		wantStart, wantEnd            time.Time
		wantLabel, wantPrev, wantNext string
	}{
		{"week wednesday anchor", RangeWeek, "2026-10-28", "", "",
			d(2026, 10, 26), d(2026, 11, 2), "10.26 - 11.01", "2026-10-19", "2026-11-02"},
		{"week sunday belongs to current week", RangeWeek, "2026-10-25", "", "",
			d(2026, 10, 19), d(2026, 10, 26), "10.19 - 10.25", "2026-10-12", "2026-10-26"},
		{"month", RangeMonth, "2026-10-15", "", "",
			d(2026, 10, 1), d(2026, 11, 1), "2026年10月", "2026-09-01", "2026-11-01"},
		{"month across year", RangeMonth, "2026-01-05", "", "",
			d(2026, 1, 1), d(2026, 2, 1), "2026年01月", "2025-12-01", "2026-02-01"},
		{"year", RangeYear, "2026-06-01", "", "",
			d(2026, 1, 1), d(2027, 1, 1), "2026年", "2025-01-01", "2027-01-01"},
		{"custom inclusive end", RangeCustom, "", "2026-10-01", "2026-10-05",
			d(2026, 10, 1), d(2026, 10, 6), "2026-10-01 - 2026-10-05", "", ""},
		{"custom single day", RangeCustom, "", "2026-10-05", "2026-10-05",
			d(2026, 10, 5), d(2026, 10, 6), "2026-10-05 - 2026-10-05", "", ""},
	}
	for _, c := range cases {
		r, err := NewRange(c.kind, c.ref, c.start, c.end, d(2026, 10, 28))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !r.Start.Equal(c.wantStart) || !r.End.Equal(c.wantEnd) || r.Label != c.wantLabel ||
			r.PrevRef != c.wantPrev || r.NextRef != c.wantNext {
			t.Fatalf("%s: got %+v", c.name, r)
		}
	}
	all, err := NewRange(RangeAll, "", "", "", d(2026, 10, 28))
	if err != nil || !all.Start.IsZero() || !all.End.IsZero() || all.Label != "全部时间" {
		t.Fatalf("all range: %+v %v", all, err)
	}
	for _, bad := range []struct{ kind, ref, start, end string }{
		{RangeWeek, "2026-13-01", "", ""},
		{RangeCustom, "", "2026-10-05", "2026-10-01"},
		{RangeCustom, "", "2026-13-01", "2026-10-05"},
		{"quarter", "", "", ""},
	} {
		if _, err := NewRange(bad.kind, bad.ref, bad.start, bad.end, d(2026, 10, 28)); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("invalid range accepted: %+v", bad)
		}
	}
}

func TestStatsFromMonthlyBudgetView(t *testing.T) {
	entries := []Entry{
		{Kind: "expense", CategoryID: 2, Amount: 2000, OccurredAt: d(2026, 10, 5)},
		{Kind: "income", CategoryID: 13, Amount: 3000, OccurredAt: d(2026, 10, 10)},
		{Kind: "expense", CategoryID: 3, Amount: 600, OccurredAt: d(2026, 10, 20)},
	}
	r, err := NewRange(RangeMonth, "2026-10-15", "", "", d(2026, 10, 26))
	if err != nil {
		t.Fatal(err)
	}
	total := int64(10000)
	s := StatsFrom(r, entries, &total, d(2026, 10, 26))
	if s.Summary.Income != 3000 || s.Summary.Expense != 2600 || s.Summary.Balance != 400 || s.Summary.DailyAvgExpense != 100 {
		t.Fatalf("summary: %+v", s.Summary)
	}
	if s.Start != "2026-10-01" || s.End != "2026-10-31" || len(s.Series) != 31 {
		t.Fatalf("range fields: %s %s %d points", s.Start, s.End, len(s.Series))
	}
	if s.Budget == nil || s.Budget.Total == nil || *s.Budget.Total != 10000 ||
		s.Budget.Remaining == nil || *s.Budget.Remaining != 7400 ||
		s.Budget.RemainingDaily == nil || *s.Budget.RemainingDaily != 1233 {
		t.Fatalf("budget view: %+v", s.Budget)
	}
}

func TestStatsFromEdgeCases(t *testing.T) {
	entries := []Entry{{Kind: "expense", CategoryID: 2, Amount: 3100, OccurredAt: d(2026, 10, 5)}}
	r, err := NewRange(RangeMonth, "2026-10-15", "", "", d(2026, 11, 10))
	if err != nil {
		t.Fatal(err)
	}
	// 期间已过：日均按整月天数，预算不给出剩余日均。
	total := int64(10000)
	s := StatsFrom(r, entries, &total, d(2026, 11, 10))
	if s.Summary.DailyAvgExpense != 100 || s.Budget.RemainingDaily != nil {
		t.Fatalf("past month: %+v", s)
	}
	// 超支：剩余为负，不出剩余日均。
	s = StatsFrom(r, entries, &[]int64{1000}[0], d(2026, 10, 26))
	if s.Budget.Remaining == nil || *s.Budget.Remaining != -2100 || s.Budget.RemainingDaily != nil {
		t.Fatalf("overspent: %+v", s.Budget)
	}
	// 未设置预算：预算视图存在但字段为空。
	s = StatsFrom(r, entries, nil, d(2026, 10, 26))
	if s.Budget == nil || s.Budget.Total != nil || s.Budget.Remaining != nil {
		t.Fatalf("unset budget: %+v", s.Budget)
	}
	// all 档：无账单时输出当年一个序列点，start 为空，日均按 1 天计。
	r, err = NewRange(RangeAll, "", "", "", d(2026, 10, 26))
	if err != nil {
		t.Fatal(err)
	}
	s = StatsFrom(r, nil, nil, d(2026, 10, 26))
	if s.Start != "" || len(s.Series) != 1 || s.Series[0].Key != "2026" || s.Summary.DailyAvgExpense != 0 {
		t.Fatalf("all range empty: %+v", s)
	}
	// 周档 7 个序列点，年档 12 个。
	r, err = NewRange(RangeWeek, "2026-10-28", "", "", d(2026, 10, 26))
	if err != nil {
		t.Fatal(err)
	}
	if s = StatsFrom(r, entries, nil, d(2026, 10, 26)); len(s.Series) != 7 {
		t.Fatal("week series length")
	}
	r, err = NewRange(RangeYear, "2026-06-01", "", "", d(2026, 10, 26))
	if err != nil {
		t.Fatal(err)
	}
	if s = StatsFrom(r, entries, nil, d(2026, 10, 26)); len(s.Series) != 12 {
		t.Fatal("year series length")
	}
}

func TestBudgetRowAndPeriodBounds(t *testing.T) {
	start, end, err := PeriodBounds(BudgetScopeMonth, "2026-10")
	if err != nil || !start.Equal(d(2026, 10, 1)) || !end.Equal(d(2026, 11, 1)) {
		t.Fatalf("month bounds: %v %v %v", start, end, err)
	}
	if start, end, err = PeriodBounds(BudgetScopeYear, "2026"); err != nil || !start.Equal(d(2026, 1, 1)) || !end.Equal(d(2027, 1, 1)) {
		t.Fatalf("year bounds: %v %v %v", start, end, err)
	}
	for period, want := range map[string]time.Time{
		"2026-W01": d(2025, 12, 29),
		"2026-W40": d(2026, 9, 28),
		"2026-W53": d(2026, 12, 28),
	} {
		start, end, err := PeriodBounds(BudgetScopeWeek, period)
		if err != nil || !start.Equal(want) || !end.Equal(want.AddDate(0, 0, 7)) {
			t.Fatalf("week bounds %s: %v %v %v", period, start, end, err)
		}
	}
	for _, bad := range []struct{ scope, period string }{
		{BudgetScopeMonth, "2026-13"}, {BudgetScopeMonth, "2026-1"}, {BudgetScopeMonth, "26-10"},
		{BudgetScopeYear, "26"}, {BudgetScopeYear, "2026-1"},
		{BudgetScopeWeek, "2026-W00"}, {BudgetScopeWeek, "2026-W54"}, {BudgetScopeWeek, "2025-W53"},
		{BudgetScopeWeek, "garbage"},
		{"quarter", "2026-Q4"},
	} {
		if _, _, err := PeriodBounds(bad.scope, bad.period); !errors.Is(err, ErrInvalidBudget) {
			t.Fatalf("invalid period accepted: %+v", bad)
		}
	}
	valid := []BudgetRow{
		{Scope: BudgetScopeMonth, Period: "2026-10", CategoryID: 0, Amount: 100},
		{Scope: BudgetScopeWeek, Period: "2026-W40", CategoryID: 3, Amount: 0},
		{Scope: BudgetScopeYear, Period: "2026", CategoryID: 0, Amount: 100000000000},
	}
	for _, row := range valid {
		if err := ValidateBudgetRow(row); err != nil {
			t.Fatalf("valid row rejected: %+v %v", row, err)
		}
	}
	for _, bad := range []BudgetRow{
		{Scope: "day", Period: "2026-10-01", CategoryID: 0, Amount: 100},
		{Scope: BudgetScopeMonth, Period: "2026-10", CategoryID: -1, Amount: 100},
		{Scope: BudgetScopeMonth, Period: "2026-10", CategoryID: 0, Amount: -1},
		{Scope: BudgetScopeMonth, Period: "2026-10", CategoryID: 0, Amount: 100000000001},
		{Scope: BudgetScopeMonth, Period: "", CategoryID: 0, Amount: 100},
		{Scope: BudgetScopeMonth, Period: "12345678901234567", CategoryID: 0, Amount: 100},
	} {
		if err := ValidateBudgetRow(bad); !errors.Is(err, ErrInvalidBudget) {
			t.Fatalf("invalid row accepted: %+v", bad)
		}
	}
}

func TestDefaultWeekStartsAtMidnight(t *testing.T) {
	now := time.Date(2026, 10, 28, 15, 42, 10, 0, cst())
	r, err := NewRange(RangeWeek, "", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 26, 0, 0, 0, 0, cst())
	if !r.Start.Equal(want) || !r.End.Equal(want.AddDate(0, 0, 7)) {
		t.Fatalf("week bounds: %v - %v", r.Start, r.End)
	}
}
