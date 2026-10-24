// Package domain 定义家庭账本的业务数据、校验规则与收支汇总逻辑。
// 金额统一使用整数分；成员身份和操作权限由接口层检查。
package domain

import (
	"errors"
	"strings"
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
	SortOrder int    `json:"sort_order"` // 展示排序值，非负整数。
	Disabled  bool   `json:"disabled"`   // 是否停用；保留已有记录对分类的引用。
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // income 为收入，expense 为支出。
	Icon      string `json:"icon"`
}

// Entry 表示一笔收入或支出；账户间转账单独使用 Transfer。
type Entry struct {
	AccountID  int64     `json:"account_id"` // 归属的资金账户 ID；0 表示未关联账户。
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
	if e.Amount <= 0 || e.Amount > 100000000000 || (e.OccurredAt.IsZero() || e.OccurredAt.Year() < 1000 || e.OccurredAt.Year() > 9999) || len([]rune(e.Note)) > 500 || len(e.RequestID) == 0 || len(e.RequestID) > 64 {
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
		{ID: 1, Name: "早餐", Kind: "expense", Icon: "🍞"}, {ID: 2, Name: "午餐", Kind: "expense", Icon: "🍜"}, {ID: 3, Name: "晚餐", Kind: "expense", Icon: "🍚"},
		{ID: 4, Name: "饮料咖啡", Kind: "expense", Icon: "☕"}, {ID: 5, Name: "交通", Kind: "expense", Icon: "🚇"}, {ID: 6, Name: "购物", Kind: "expense", Icon: "🛍️"},
		{ID: 7, Name: "住房", Kind: "expense", Icon: "🏠"}, {ID: 8, Name: "其他支出", Kind: "expense", Icon: "🧾"},
		{ID: 9, Name: "工资", Kind: "income", Icon: "💼"}, {ID: 10, Name: "奖金", Kind: "income", Icon: "🎁"}, {ID: 11, Name: "其他收入", Kind: "income", Icon: "💰"},
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

// Account 表示现金、微信等资金账户，金额单位为分。
type Account struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`                 // cash、wechat、alipay、bank 或 other。
	OpeningBalance int64  `json:"opening_balance_cent"` // 期初余额，可为负数。
	Balance        int64  `json:"balance_cent"`         // 期初余额加收入、减支出，再加转入、减转出。
	Disabled       bool   `json:"disabled"`             // 是否停用；保留已有记账和转账记录。
}

// Transfer 表示两个不同资金账户之间的转账，影响账户余额，不计入收入或支出。
type Transfer struct {
	ID            int64     `json:"id"`
	FromAccountID int64     `json:"from_account_id"` // 转出账户 ID。
	ToAccountID   int64     `json:"to_account_id"`   // 转入账户 ID，必须与转出账户不同。
	Amount        int64     `json:"amount_cent"`     // 转账金额，单位为分，必须为正数。
	OccurredAt    time.Time `json:"occurred_at"`     // 转账的业务发生时间。
	Note          string    `json:"note"`
	RequestID     string    `json:"client_request_id"` // 转账创建请求的客户端幂等键。
	CreatedBy     int64     `json:"created_by"`        // 创建转账的成员 ID。
}

// EntryFilter 表示记账查询条件；各条件同时满足才匹配。
// 指针为 nil 表示不筛选该字段，非 nil 的零值仍参与筛选。
type EntryFilter struct {
	// Start 包含边界，End 不包含边界；零值表示该侧不限制。
	Start, End time.Time
	// Query 为备注中区分大小写的字面子串；Kind 为 income 或 expense；空字符串表示不限制。
	Query, Kind string
	// 各 ID 用于精确匹配；CreatedBy、AccountID 指向 0 分别筛选历史无创建者、未关联账户记录。
	// MinAmount、MaxAmount 的单位为分，均包含边界。
	CategoryID, CreatedBy, AccountID, MinAmount, MaxAmount *int64
	// Page 从 1 开始，PageSize 为每页条数；两者不参与 Match 判断。
	Page, PageSize int
}

// EntryPage 表示记账查询的分页结果。
type EntryPage struct {
	Items    []Entry `json:"items"`
	Total    int     `json:"total"` // 满足筛选条件的记录总数。
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Summary  Summary `json:"summary"`         // 所有匹配记录的汇总，不限于当前页。
	Month    string  `json:"month,omitempty"` // 按月查询时为 YYYY-MM；按日期范围查询时省略。
}

// TransferPage 表示转账记录的分页结果。
type TransferPage struct {
	Items    []Transfer `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// Author 表示记账记录中出现过的创建者，用于创建者筛选选项。
type Author struct {
	ID int64 `json:"id"` // 成员 ID；0 表示历史无创建者记录。
}

// ValidateCategory 校验分类名称、图标长度、排序值和收支类型。
func ValidateCategory(c Category) error {
	if strings.TrimSpace(c.Name) == "" || len([]rune(c.Name)) > 100 || len([]rune(c.Icon)) > 32 || c.SortOrder < 0 || c.SortOrder > 1000000 || (c.Kind != "expense" && c.Kind != "income") {
		return ErrInvalid
	}
	return nil
}

// ValidateAccount 校验账户名称、期初余额范围和账户类型。
func ValidateAccount(a Account) error {
	if strings.TrimSpace(a.Name) == "" || len([]rune(a.Name)) > 100 || a.OpeningBalance < -100000000000 || a.OpeningBalance > 100000000000 {
		return ErrInvalid
	}
	switch a.Type {
	case "cash", "wechat", "alipay", "bank", "other":
		return nil
	}
	return ErrInvalid
}

// ValidateTransfer 校验转账账户 ID、金额、发生时间、备注和幂等键。
// 账户是否存在、是否停用及操作权限由调用方检查。
func ValidateTransfer(t Transfer) error {
	if t.FromAccountID <= 0 || t.ToAccountID <= 0 || t.FromAccountID == t.ToAccountID || t.Amount <= 0 || t.Amount > 100000000000 || t.OccurredAt.IsZero() || t.OccurredAt.Year() < 1000 || t.OccurredAt.Year() > 9999 || len([]rune(t.Note)) > 500 || len(t.RequestID) == 0 || len(t.RequestID) > 64 {
		return ErrInvalid
	}
	return nil
}

// Match 判断记录是否满足筛选条件，不处理分页。
func (f EntryFilter) Match(e Entry) bool {
	return (f.Start.IsZero() || !e.OccurredAt.Before(f.Start)) && (f.End.IsZero() || e.OccurredAt.Before(f.End)) && strings.Contains(e.Note, f.Query) && (f.Kind == "" || e.Kind == f.Kind) && (f.CategoryID == nil || e.CategoryID == *f.CategoryID) && (f.CreatedBy == nil || e.CreatedBy == *f.CreatedBy) && (f.AccountID == nil || e.AccountID == *f.AccountID) && (f.MinAmount == nil || e.Amount >= *f.MinAmount) && (f.MaxAmount == nil || e.Amount <= *f.MaxAmount)
}

// CategoryPatch 表示分类的部分更新，字段含义与 Category 一致。
// nil 表示不修改（JSON 字段省略或为 null）；非 nil 的零值和 false 表示显式更新。
type CategoryPatch struct {
	Name      *string `json:"name"`
	Kind      *string `json:"kind"`
	Icon      *string `json:"icon"`
	SortOrder *int    `json:"sort_order"`
	Disabled  *bool   `json:"disabled"`
}

// Apply 将非 nil 字段应用到分类副本，并去除名称首尾空白；调用方负责校验结果。
func (p CategoryPatch) Apply(c Category) Category {
	if p.Name != nil {
		c.Name = strings.TrimSpace(*p.Name)
	}
	if p.Kind != nil {
		c.Kind = *p.Kind
	}
	if p.Icon != nil {
		c.Icon = *p.Icon
	}
	if p.SortOrder != nil {
		c.SortOrder = *p.SortOrder
	}
	if p.Disabled != nil {
		c.Disabled = *p.Disabled
	}
	return c
}

// AccountPatch 表示账户的部分更新，字段含义与 Account 一致。
// nil 表示不修改；期初余额指向 0、停用状态指向 false 均表示显式更新。
type AccountPatch struct {
	Name           *string `json:"name"`
	Type           *string `json:"type"`
	OpeningBalance *int64  `json:"opening_balance_cent"`
	Disabled       *bool   `json:"disabled"`
}

// Apply 将非 nil 字段应用到账户副本，并去除名称首尾空白；不校验或重算余额。
func (p AccountPatch) Apply(a Account) Account {
	if p.Name != nil {
		a.Name = strings.TrimSpace(*p.Name)
	}
	if p.Type != nil {
		a.Type = *p.Type
	}
	if p.OpeningBalance != nil {
		a.OpeningBalance = *p.OpeningBalance
	}
	if p.Disabled != nil {
		a.Disabled = *p.Disabled
	}
	return a
}
