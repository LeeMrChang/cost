package domain

import (
	_ "embed"
	"encoding/json"
)

//go:embed category_icon_catalog.json
var categoryIconJSON []byte

type CategoryIcon struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Emoji    string   `json:"emoji"`
	Old      []string `json:"old"`
	SystemID int64    `json:"system_id"`
	Kind     string   `json:"kind"`
}

var categoryIconCatalog = func() []CategoryIcon {
	var v []CategoryIcon
	if err := json.Unmarshal(categoryIconJSON, &v); err != nil {
		panic(err)
	}
	return v
}()

func CategoryIcons() []CategoryIcon { return append([]CategoryIcon(nil), categoryIconCatalog...) }
func ValidIconKey(key string) bool {
	if key == "" {
		return true
	}
	for _, i := range categoryIconCatalog {
		if i.Key == key {
			return true
		}
	}
	return false
}
func (i CategoryIcon) IsDefaultEmoji(s string) bool {
	if s == i.Emoji {
		return true
	}
	for _, v := range i.Old {
		if s == v {
			return true
		}
	}
	return false
}

// IconKeyPatch distinguishes an omitted key from an explicit empty string or null.
type IconKeyPatch struct {
	Set   bool
	Value string
}

func (p *IconKeyPatch) UnmarshalJSON(b []byte) error {
	var v *string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	p.Set = true
	p.Value = ""
	if v != nil {
		p.Value = *v
	}
	return nil
}
