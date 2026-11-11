package domain

import "testing"

func TestDefaultCategoryIcons(t *testing.T) {
	want := map[string]string{
		"expense/餐饮": "🍜", "expense/交通": "🚗", "expense/夜宵": "🍗", "expense/公交": "🚌", "expense/地铁": "🚇", "expense/早餐": "🥐", "expense/午餐": "🍛", "expense/晚餐": "🍲", "expense/消费": "🛒", "expense/饮料咖啡": "☕", "expense/住房": "🏠", "expense/其他": "📦",
		"income/工资": "💼", "income/奖金": "🏆", "income/加班": "⏰", "income/福利": "🎫", "income/公积金": "🏦", "income/红包": "🧧", "income/兼职": "💻", "income/副业": "🌱", "income/退税": "💸", "income/投资": "📊", "income/意外收入": "🍀", "income/其他": "💰",
	}
	cats := DefaultCategories()
	if len(cats) != len(want) {
		t.Fatalf("default categories = %d, want %d", len(cats), len(want))
	}
	for _, c := range cats {
		if c.Icon != want[c.Kind+"/"+c.Name] {
			t.Errorf("%s/%s = %s, want %s", c.Kind, c.Name, c.Icon, want[c.Kind+"/"+c.Name])
		}
	}
}
