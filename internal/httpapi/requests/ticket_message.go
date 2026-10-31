package requests

import (

	"github.com/thomas666-beast/reqval"
)

type TicketMessage struct {
	reqval.Base

	Body string `json:"body"`
}

func (r *TicketMessage) Rules() map[string][]string {
	return map[string][]string{
		"Body": {"required", "min:1", "max:10000"},
	}
}
