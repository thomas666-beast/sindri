package requests

import (
	"github.com/thomas666-beast/reqval"
)

type RequestWithdrawal struct {
	reqval.Base

	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

func (r *RequestWithdrawal) Rules() map[string][]string {
	return map[string][]string{
		"Currency": {"required", "min:3", "max:3", "exists:currencies,code"},
		"Amount":   {"required", "gte:1"},
	}
}

type RejectWithdrawal struct {
	reqval.Base

	Reason string `json:"reason"`
}

func (r *RejectWithdrawal) Rules() map[string][]string {
	return map[string][]string{
		"Reason": {"required", "min:3", "max:1000"},
	}
}
