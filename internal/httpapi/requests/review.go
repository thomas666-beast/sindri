package requests

import (
	"github.com/thomas666-beast/reqval"
)

type CreateReview struct {
	reqval.Base

	OrderID string `json:"order_id"`
	Rating  int16  `json:"rating"`
	Comment string `json:"comment"`
}

func (r *CreateReview) Rules() map[string][]string {
	return map[string][]string{
		"OrderID": {"required", "uuid"},
		"Rating":  {"required", "gte:1", "lte:5"},
		"Comment": {"max:2000"},
	}
}
