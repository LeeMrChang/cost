package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("request id already used for different data")
var ErrInvalid = errors.New("invalid entry")

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Icon string `json:"icon"`
}

type Entry struct {
	CreatedBy  int64     `json:"created_by"`
	ID         int64     `json:"id"`
	CategoryID int64     `json:"category_id"`
	Kind       string    `json:"kind"`
	Amount     int64     `json:"amount_cent"`
	OccurredAt time.Time `json:"occurred_at"`
	Note       string    `json:"note"`
	RequestID  string    `json:"client_request_id"`
}

func Validate(e Entry, categories []Category) error {
	if e.Amount <= 0 || e.Amount > 100000000000 || e.OccurredAt.IsZero() || len([]rune(e.Note)) > 500 || len(e.RequestID) == 0 || len(e.RequestID) > 64 {
		return ErrInvalid
	}
	for _, c := range categories {
		if c.ID == e.CategoryID && c.Kind == e.Kind {
			return nil
		}
	}
	return ErrInvalid
}

func DefaultCategories() []Category {
	return []Category{
		{1, "早餐", "expense", "🍞"}, {2, "午餐", "expense", "🍜"}, {3, "晚餐", "expense", "🍚"},
		{4, "饮料咖啡", "expense", "☕"}, {5, "交通", "expense", "🚇"}, {6, "购物", "expense", "🛍️"},
		{7, "住房", "expense", "🏠"}, {8, "其他支出", "expense", "🧾"},
		{9, "工资", "income", "💼"}, {10, "奖金", "income", "🎁"}, {11, "其他收入", "income", "💰"},
	}
}

// Bounds defines a calendar month in the ledger timezone, using [start, end).
func Bounds(month string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, 0), nil
}

type CategoryTotal struct {
	CategoryID int64 `json:"category_id"`
	Amount     int64 `json:"amount_cent"`
	Count      int   `json:"count"`
}
type DayTotal struct {
	Date    string `json:"date"`
	Expense int64  `json:"expense_cent"`
	Income  int64  `json:"income_cent"`
}
type Summary struct {
	Income     int64           `json:"income_cent"`
	Expense    int64           `json:"expense_cent"`
	Balance    int64           `json:"balance_cent"`
	Budget     *int64          `json:"budget_cent"`
	Remaining  *int64          `json:"remaining_cent"`
	Categories []CategoryTotal `json:"categories"`
	Days       []DayTotal      `json:"days"`
}

func Summarize(entries []Entry, budget *int64) Summary {
	s := Summary{Budget: budget, Categories: []CategoryTotal{}, Days: []DayTotal{}}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	cats := map[int64]int{}
	days := map[string]int{}
	for _, e := range entries {
		date := e.OccurredAt.In(loc).Format("2006-01-02")
		i, ok := days[date]
		if !ok {
			i = len(s.Days)
			days[date] = i
			s.Days = append(s.Days, DayTotal{Date: date})
		}
		if e.Kind == "income" {
			s.Income += e.Amount
			s.Days[i].Income += e.Amount
			continue
		}
		s.Expense += e.Amount
		s.Days[i].Expense += e.Amount
		j, ok := cats[e.CategoryID]
		if !ok {
			j = len(s.Categories)
			cats[e.CategoryID] = j
			s.Categories = append(s.Categories, CategoryTotal{CategoryID: e.CategoryID})
		}
		s.Categories[j].Amount += e.Amount
		s.Categories[j].Count++
	}
	s.Balance = s.Income - s.Expense
	if budget != nil {
		r := *budget - s.Expense
		s.Remaining = &r
	}
	return s
}
