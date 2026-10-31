package requests

import (
	"github.com/thomas666-beast/reqval"
)

type CreateInvoice struct {
	reqval.Base

	BuyerUsername string `json:"buyer_username"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
}

func (r *CreateInvoice) Rules() map[string][]string {
	return map[string][]string{
		"BuyerUsername": {"required", "min:3", "max:32"},
		"Title":         {"required", "min:1", "max:200"},
		"Amount":        {"required", "gte:1"},
		"Currency":      {"required", "min:3", "max:3", "exists:currencies,code"},
	}
}
