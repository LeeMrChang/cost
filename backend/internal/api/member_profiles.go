package api

import (
	"context"
	"cost/internal/domain"
	"fmt"
	"sort"
)

// authorDirectory 合并当前成员和本账本历史创建人，避免泄漏从未加入家庭的用户。
func (a *API) authorDirectory(ctx context.Context) ([]domain.Author, error) {
	history, err := a.store.Authors(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]domain.Author{}
	ids := []int64{}
	for _, v := range history {
		v.Status = "removed"
		if v.ID == 0 {
			v.Status = "unspecified"
		}
		byID[v.ID] = v
		ids = append(ids, v.ID)
	}
	if a.profiles != nil {
		profiles, err := a.profiles.Profiles(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			status := "removed"
			if p.Role != "" {
				status = "active"
			}
			byID[p.ID] = domain.Author{ID: p.ID, Nickname: p.Nickname, Status: status}
		}
	}
	out := []domain.Author{}
	for _, p := range byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func authorDisplay(p domain.Author) (string, string) {
	if p.ID == 0 {
		return "历史未指定成员", "历史未指定成员"
	}
	name := p.Nickname
	if name == "" {
		name = fmt.Sprintf("成员 %d", p.ID)
	}
	if p.Status == "removed" {
		return name + "（已移除）", "已移除"
	}
	return name, "在家成员"
}
