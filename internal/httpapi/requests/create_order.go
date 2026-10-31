package requests

import (

	"github.com/thomas666-beast/reqval"
)

type CreateOrder struct {
	reqval.Base

	ProductID string `json:"product_id"`
}

func (r *CreateOrder) Rules() map[string][]string {
	return map[string][]string{
		"ProductID": {"required", "uuid"},
	}
}
