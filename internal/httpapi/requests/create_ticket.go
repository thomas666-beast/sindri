package requests

import (

	"github.com/thomas666-beast/reqval"
)

type CreateTicket struct {
	reqval.Base

	Subject  string `json:"subject"`
	Priority string `json:"priority"`
}

func (r *CreateTicket) Rules() map[string][]string {
	return map[string][]string{
		"Subject":  {"required", "min:1", "max:300"},
		"Priority": {"required", "in:low,normal,high"},
	}
}
