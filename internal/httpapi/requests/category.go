package requests

import (
	"github.com/thomas666-beast/reqval"
)

type CreateCategory struct {
	reqval.Base

	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
	ParentID    string `json:"parent_id"`
}

func (r *CreateCategory) Rules() map[string][]string {
	return map[string][]string{
		"Slug": {"required", "min:2", "max:64", "regexp:^[a-z0-9-]+$"},
		"Name": {"required", "min:1", "max:100"},
	}
}

type UpdateCategory struct {
	reqval.Base

	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
	IsActive    *bool  `json:"is_active"`
}

func (r *UpdateCategory) Rules() map[string][]string {
	return map[string][]string{
		"Slug": {"required", "min:2", "max:64", "regexp:^[a-z0-9-]+$"},
		"Name": {"required", "min:1", "max:100"},
	}
}
