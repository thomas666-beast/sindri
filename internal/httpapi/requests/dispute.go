package requests

import (
	"github.com/thomas666-beast/reqval"
)

type DisputeOrder struct {
	reqval.Base

	Reason string `json:"reason"`
}

func (r *DisputeOrder) Rules() map[string][]string {
	return map[string][]string{
		"Reason": {"required", "min:3", "max:2000"},
	}
}
