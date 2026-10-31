package requests

import (
	"github.com/thomas666-beast/reqval"
)

type CreateProduct struct {
	reqval.Base

	Title        string `json:"title"`
	Description  string `json:"description"`
	Price        int64  `json:"price"`
	CurrencyCode string `json:"currency_code"`
	CategoryID   string `json:"category_id"`
}

func (r *CreateProduct) Rules() map[string][]string {
	return map[string][]string{
		"Title":        {"required", "min:1", "max:200"},
		"Price":        {"required", "gte:1"},
		"CurrencyCode": {"required", "min:3", "max:3", "exists:currencies,code"},
	}
}

type SetProductCategory struct {
	reqval.Base

	CategoryID string `json:"category_id"`
}

func (r *SetProductCategory) Rules() map[string][]string {
	// Empty category_id is valid — it removes the category.
	return map[string][]string{}
}
