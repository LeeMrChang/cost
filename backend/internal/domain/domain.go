// Package domain 定义家庭账本的业务数据、校验规则与收支汇总逻辑。
// 金额统一使用整数分；成员身份和操作权限由接口层检查。
package domain

import (
	"errors"
	"time"
)

// ErrNotFound 表示请求的业务记录不存在。
var ErrNotFound = errors.New("not found")

// ErrConflict 表示客户端幂等键已被其他内容占用。
var ErrConflict = errors.New("request id already used for different data")

// ErrInvalid 表示业务数据未通过校验。
var ErrInvalid = errors.New("invalid entry")

// Category 表示收入或支出的分类。
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // income 为收入，expense 为支出。
	Icon string `json:"icon"`
}

// Entry 表示一笔收入或支出记录。
type Entry struct {
	CreatedBy  int64     `json:"created_by"` // 创建者的成员 ID；历史无创建者记录为 0。
	ID         int64     `json:"id"`
	CategoryID int64     `json:"category_id"`
	Kind       string    `json:"kind"`        // income 或 expense，必须与分类的收支类型一致。
	Amount     int64     `json:"amount_cent"` // 金额，单位为分，必须为正数；收支方向由 Kind 决定。
	OccurredAt time.Time `json:"occurred_at"` // 业务发生时间；按北京时间归入统计日期和月份。
	Note       string    `json:"note"`
	RequestID  string    `json:"client_request_id"` // 客户端幂等键，用于识别重复创建请求。
}

// Validate 校验记账金额、发生时间、备注、幂等键及分类类型匹配。
// 账户可用性、成员身份和编辑权限不在此函数中校验。
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

// DefaultCategories 返回账本初始化使用的默认收支分类。
func DefaultCategories() []Category {
	return []Category{
		{1, "早餐", "expense", "🍞"}, {2, "午餐", "expense", "🍜"}, {3, "晚餐", "expense", "🍚"},
		{4, "饮料咖啡", "expense", "☕"}, {5, "交通", "expense", "🚇"}, {6, "购物", "expense", "🛍️"},
		{7, "住房", "expense", "🏠"}, {8, "其他支出", "expense", "🧾"},
		{9, "工资", "income", "💼"}, {10, "奖金", "income", "🎁"}, {11, "其他收入", "income", "💰"},
	}
}

// Bounds 将 YYYY-MM 月份转换为 Asia/Shanghai 时区的自然月范围。
// 返回的时间区间为 [start, end)，包含月初，不包含下月月初。
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

// CategoryTotal 表示某个支出分类的汇总，不包含收入分类。
type CategoryTotal struct {
	CategoryID int64 `json:"category_id"`
	Amount     int64 `json:"amount_cent"` // 该分类的支出总额，单位为分。
	Count      int   `json:"count"`       // 该分类的支出记录笔数。
}

// DayTotal 表示北京时间某一天的收支汇总，金额单位为分。
type DayTotal struct {
	Date    string `json:"date"` // 日期格式为 YYYY-MM-DD。
	Expense int64  `json:"expense_cent"`
	Income  int64  `json:"income_cent"`
}

// Summary 表示所选记账记录的收支汇总，所有金额单位为分，不包含转账。
type Summary struct {
	Income     int64           `json:"income_cent"`
	Expense    int64           `json:"expense_cent"`
	Balance    int64           `json:"balance_cent"`   // 收入减支出，可为负数；不是资金账户余额。
	Budget     *int64          `json:"budget_cent"`    // nil 表示未设置预算；指向 0 表示预算为零。
	Remaining  *int64          `json:"remaining_cent"` // 预算减支出；无预算时为 nil，超支时为负数。
	Categories []CategoryTotal `json:"categories"`     // 按分类汇总的支出。
	Days       []DayTotal      `json:"days"`
}

// Summarize 汇总传入的已校验收支记录，按北京时间聚合每日收支及分类支出。
// 调用方负责筛选统计范围并传入对应预算；本函数不筛选月份、不排序、不补齐无记录日期。
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
