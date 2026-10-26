package domain

import (
	"errors"
	"fmt"
	"time"
)

// 统计与预算区间：五档口径（周/月/年/全部/自定义范围），周以周一为起点，日界按 Asia/Shanghai。

const (
	RangeWeek   = "week"
	RangeMonth  = "month"
	RangeYear   = "year"
	RangeAll    = "all"
	RangeCustom = "custom"
)

var ErrInvalidRange = errors.New("invalid range")

// Range 表示一个统计区间；Start 零值表示无下界（all），End 为开区间上界。
type Range struct {
	Kind    string
	Start   time.Time
	End     time.Time
	Label   string
	Ref     string // 当前锚点日期 YYYY-MM-DD（week/month/year 用于切期）
	PrevRef string // 上一期锚点 YYYY-MM-DD；custom/all 为空
	NextRef string // 下一期锚点 YYYY-MM-DD；custom/all 为空
}

func cst() *time.Location {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return loc
}

func parseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, cst())
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, ErrInvalidRange
	}
	return t, nil
}

func dateOf(t time.Time) string { return t.In(cst()).Format("2006-01-02") }

func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // days since Monday
	return t.AddDate(0, 0, -offset)
}

// NewRange 构造统计区间。kind 为 week/month/year/all/custom；
// ref 为锚点日期（默认今天，本地时间）；start/end 仅 custom 使用（YYYY-MM-DD，含尾日）。
func NewRange(kind, ref, start, end string, today time.Time) (Range, error) {
	now := today
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(cst())
	anchor := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cst())
	if ref != "" {
		t, err := parseDate(ref)
		if err != nil {
			return Range{}, err
		}
		anchor = t
	}
	switch kind {
	case RangeWeek:
		start := mondayOf(anchor)
		return Range{
			Kind:    kind,
			Start:   start,
			End:     start.AddDate(0, 0, 7),
			Label:   start.Format("01.02") + " - " + start.AddDate(0, 0, 6).Format("01.02"),
			Ref:     dateOf(start),
			PrevRef: dateOf(start.AddDate(0, 0, -7)),
			NextRef: dateOf(start.AddDate(0, 0, 7)),
		}, nil
	case RangeMonth:
		start := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, cst())
		return Range{
			Kind:    kind,
			Start:   start,
			End:     start.AddDate(0, 1, 0),
			Label:   start.Format("2006年01月"),
			Ref:     dateOf(start),
			PrevRef: dateOf(start.AddDate(0, -1, 0)),
			NextRef: dateOf(start.AddDate(0, 1, 0)),
		}, nil
	case RangeYear:
		start := time.Date(anchor.Year(), 1, 1, 0, 0, 0, 0, cst())
		return Range{
			Kind:    kind,
			Start:   start,
			End:     start.AddDate(1, 0, 0),
			Label:   start.Format("2006年"),
			Ref:     dateOf(start),
			PrevRef: dateOf(start.AddDate(-1, 0, 0)),
			NextRef: dateOf(start.AddDate(1, 0, 0)),
		}, nil
	case RangeAll:
		return Range{Kind: kind, Label: "全部时间"}, nil
	case RangeCustom:
		s, err := parseDate(start)
		if err != nil {
			return Range{}, err
		}
		e, err := parseDate(end)
		if err != nil {
			return Range{}, err
		}
		if s.After(e) {
			return Range{}, ErrInvalidRange
		}
		return Range{Kind: kind, Start: s, End: e.AddDate(0, 0, 1), Label: s.Format("2006-01-02") + " - " + e.Format("2006-01-02"), Ref: dateOf(s)}, nil
	}
	return Range{}, ErrInvalidRange
}

type StatsSummary struct {
	Income          int64 `json:"income_cent"`
	Expense         int64 `json:"expense_cent"`
	Balance         int64 `json:"balance_cent"`
	DailyAvgExpense int64 `json:"daily_avg_expense_cent"`
}

type SeriesPoint struct {
	Key     string `json:"key"` // 天：YYYY-MM-DD；月：YYYY-MM；年：YYYY
	Expense int64  `json:"expense_cent"`
	Income  int64  `json:"income_cent"`
}

type BudgetView struct {
	Total          *int64 `json:"total_cent"`           // nil 未设置总预算
	Remaining      *int64 `json:"remaining_cent"`       // 总预算减支出，超支为负
	RemainingDaily *int64 `json:"remaining_daily_cent"` // 剩余÷本月剩余天数；期间已过或剩余<=0 时为 nil
}

type Stats struct {
	Range      string          `json:"range"`
	Label      string          `json:"label"`
	Start      string          `json:"start,omitempty"`
	End        string          `json:"end,omitempty"` // 开区间上界日期
	PrevRef    string          `json:"prev_ref,omitempty"`
	NextRef    string          `json:"next_ref,omitempty"`
	Summary    StatsSummary    `json:"summary"`
	Series     []SeriesPoint   `json:"series"`
	Categories []CategoryTotal `json:"categories"`
	Budget     *BudgetView     `json:"budget,omitempty"` // 仅月档返回
}

func elapsedDays(start, today time.Time) int64 {
	d := int64(today.Sub(start).Hours()/24) + 1
	if d < 1 {
		d = 1
	}
	return d
}

func remainingDaysOfMonth(today, end time.Time) int64 {
	return int64(end.Sub(today).Hours() / 24)
}

// StatsFrom 汇总区间内账单并生成五档统计；budgetTotal 仅在月档传入总预算（nil 表示未设置）。
func StatsFrom(r Range, entries []Entry, budgetTotal *int64, now time.Time) Stats {
	s := Summarize(entries, budgetTotal)
	out := Stats{Range: r.Kind, Label: r.Label, PrevRef: r.PrevRef, NextRef: r.NextRef,
		Summary: StatsSummary{Income: s.Income, Expense: s.Expense, Balance: s.Balance},
		Series:  []SeriesPoint{}, Categories: s.Categories}
	today := now
	if today.IsZero() {
		today = time.Now()
	}
	today = today.In(cst())
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, cst())
	days := int64(0)
	switch {
	case r.Start.IsZero():
		if len(entries) > 0 {
			first := entries[len(entries)-1].OccurredAt
			for _, e := range entries {
				if e.OccurredAt.Before(first) {
					first = e.OccurredAt
				}
			}
			days = elapsedDays(time.Date(first.In(cst()).Year(), first.In(cst()).Month(), first.In(cst()).Day(), 0, 0, 0, 0, cst()), today)
		} else {
			days = 1
		}
	case today.Before(r.End) && !today.Before(r.Start):
		days = elapsedDays(r.Start, today)
	default:
		days = int64(r.End.Sub(r.Start).Hours() / 24)
	}
	out.Summary.DailyAvgExpense = s.Expense / days

	spent := map[string]*SeriesPoint{}
	for _, e := range entries {
		t := e.OccurredAt.In(cst())
		var key string
		switch r.Kind {
		case RangeYear:
			key = t.Format("2006-01")
		case RangeAll:
			key = t.Format("2006")
		default:
			key = t.Format("2006-01-02")
		}
		p := spent[key]
		if p == nil {
			p = &SeriesPoint{Key: key}
			spent[key] = p
		}
		if e.Kind == "income" {
			p.Income += e.Amount
		} else {
			p.Expense += e.Amount
		}
	}
	switch r.Kind {
	case RangeAll:
		from, to := today.Year(), today.Year()
		if len(entries) > 0 {
			first, last := entries[0].OccurredAt.In(cst()), entries[0].OccurredAt.In(cst())
			for _, e := range entries {
				if t := e.OccurredAt.In(cst()); t.Year() < first.Year() {
					first = t
				} else if t.Year() > last.Year() {
					last = t
				}
			}
			from, to = first.Year(), last.Year()
		}
		for y := from; y <= to; y++ {
			key := time.Date(y, 1, 1, 0, 0, 0, 0, cst()).Format("2006")
			p := spent[key]
			if p == nil {
				p = &SeriesPoint{Key: key}
			}
			out.Series = append(out.Series, *p)
		}
	case RangeYear:
		for m := 1; m <= 12; m++ {
			key := time.Date(r.Start.Year(), time.Month(m), 1, 0, 0, 0, 0, cst()).Format("2006-01")
			p := spent[key]
			if p == nil {
				p = &SeriesPoint{Key: key}
			}
			out.Series = append(out.Series, *p)
		}
	default:
		for d := r.Start; d.Before(r.End); d = d.AddDate(0, 0, 1) {
			key := d.Format("2006-01-02")
			p := spent[key]
			if p == nil {
				p = &SeriesPoint{Key: key}
			}
			out.Series = append(out.Series, *p)
		}
	}

	if r.Kind == RangeMonth {
		view := &BudgetView{Total: budgetTotal}
		if budgetTotal != nil {
			remaining := *budgetTotal - s.Expense
			view.Remaining = &remaining
			if !today.Before(r.Start) && today.Before(r.End) {
				left := remainingDaysOfMonth(today, r.End)
				if left > 0 && remaining > 0 {
					daily := remaining / left
					view.RemainingDaily = &daily
				}
			}
		}
		out.Budget = view
	}
	if !r.Start.IsZero() {
		out.Start = dateOf(r.Start)
	}
	if !r.End.IsZero() {
		out.End = dateOf(r.End.Add(-time.Hour)) // 开区间上界回退一天用于展示
	}
	return out
}

const (
	BudgetScopeWeek  = "week"
	BudgetScopeMonth = "month"
	BudgetScopeYear  = "year"
)

// BudgetRow 表示一条预算设置；CategoryID=0 为总预算。
type BudgetRow struct {
	Scope      string `json:"scope"`
	Period     string `json:"period"`
	CategoryID int64  `json:"category_id"`
	Amount     int64  `json:"amount_cent"`
}

var ErrInvalidBudget = errors.New("invalid budget")

func ValidateBudgetRow(r BudgetRow) error {
	switch r.Scope {
	case BudgetScopeWeek, BudgetScopeMonth, BudgetScopeYear:
	default:
		return ErrInvalidBudget
	}
	if r.CategoryID < 0 || r.Amount < 0 || r.Amount > 100000000000 || len(r.Period) == 0 || len(r.Period) > 16 {
		return ErrInvalidBudget
	}
	_, _, err := PeriodBounds(r.Scope, r.Period)
	return err
}

// MonthPeriodKey 返回 month 档的 period_key（YYYY-MM）。
func MonthPeriodKey(start time.Time) string { return start.In(cst()).Format("2006-01") }

// PeriodBounds 返回预算期间的 [start, end) 范围（Asia/Shanghai）：
// 月 YYYY-MM、年 YYYY、周 YYYY-Www（ISO 周，周一为起点）。
func PeriodBounds(scope, period string) (time.Time, time.Time, error) {
	loc := cst()
	switch scope {
	case BudgetScopeMonth:
		start, err := time.ParseInLocation("2006-01", period, loc)
		if err != nil || start.Format("2006-01") != period {
			return time.Time{}, time.Time{}, ErrInvalidBudget
		}
		return start, start.AddDate(0, 1, 0), nil
	case BudgetScopeYear:
		start, err := time.ParseInLocation("2006", period, loc)
		if err != nil || start.Format("2006") != period {
			return time.Time{}, time.Time{}, ErrInvalidBudget
		}
		return start, start.AddDate(1, 0, 0), nil
	case BudgetScopeWeek:
		start, err := isoWeekStart(period)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return start, start.AddDate(0, 0, 7), nil
	}
	return time.Time{}, time.Time{}, ErrInvalidBudget
}

// isoWeekStart 解析 YYYY-Www 形式的 ISO 周，返回该周周一零点（1 月 4 日恒在 ISO 第 1 周）。
func isoWeekStart(period string) (time.Time, error) {
	var y, w int
	if _, err := fmt.Sscanf(period, "%d-W%d", &y, &w); err != nil || y < 1000 || y > 9999 || w < 1 || w > 53 {
		return time.Time{}, ErrInvalidBudget
	}
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, cst())
	t := jan4.AddDate(0, 0, 7*(w-1))
	offset := (int(t.Weekday()) + 6) % 7
	monday := t.AddDate(0, 0, -offset)
	if monday.AddDate(0, 0, 3).Year() != y { // 该周周四须落在声明年份，排除 2025-W53 这类伪周。
		return time.Time{}, ErrInvalidBudget
	}
	return monday, nil
}

// Ledger 表示单账本实体。
type Ledger struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

func DefaultLedgerName() string { return "我的账本" }
func DefaultLedgerIcon() string { return "📒" }
