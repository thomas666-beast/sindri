package requests

import (
	"github.com/thomas666-beast/reqval"
)

type UpdateProduct struct {
	reqval.Base

	Title       string `json:"title"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	CategoryID  string `json:"category_id"`
}

func (r *UpdateProduct) Rules() map[string][]string {
	return map[string][]string{
		"Title": {"required", "min:1", "max:200"},
		"Price": {"required", "gte:1"},
	}
}
