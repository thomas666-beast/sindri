package requests

import (
	"github.com/thomas666-beast/reqval"
)

type WalletOp struct {
	reqval.Base

	Currency       string `json:"currency"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (r *WalletOp) Rules() map[string][]string {
	return map[string][]string{
		"Currency": {"required", "min:3", "max:3", "exists:currencies,code"},
		"Amount":   {"required", "gte:1"},
	}
}
