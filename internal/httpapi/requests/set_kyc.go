package requests

import (

	"github.com/thomas666-beast/reqval"
)

type SetKYC struct {
	reqval.Base

	Status string `json:"status"`
}

func (r *SetKYC) Rules() map[string][]string {
	return map[string][]string{
		"Status": {"required", "in:not_required,pending,verified,rejected"},
	}
}
