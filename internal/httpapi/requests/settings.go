package requests

import (
	"github.com/thomas666-beast/reqval"
)

type UpdateSetting struct {
	reqval.Base

	Value string `json:"value"`
}

func (r *UpdateSetting) Rules() map[string][]string {
	return map[string][]string{
		"Value": {"required", "min:1", "max:255"},
	}
}
