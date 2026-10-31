package requests

import (

	"github.com/thomas666-beast/reqval"
)

type SetTicketStatus struct {
	reqval.Base

	Status string `json:"status"`
}

func (r *SetTicketStatus) Rules() map[string][]string {
	return map[string][]string{
		"Status": {"required", "in:open,answered,closed"},
	}
}
