package requests

import (
	"github.com/thomas666-beast/reqval"
)

type CancelInvoice struct {
	reqval.Base

	Reason string `json:"reason"`
}

func (r *CancelInvoice) Rules() map[string][]string {
	return map[string][]string{
		"Reason": {"required", "min:3", "max:1000"},
	}
}
