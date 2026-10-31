package requests

import (

	"github.com/thomas666-beast/reqval"
)

type DisableUser struct {
	reqval.Base

	Reason string `json:"reason"`
}

func (r *DisableUser) Rules() map[string][]string {
	return map[string][]string{
		"Reason": {"required", "min:3", "max:2000"},
	}
}
