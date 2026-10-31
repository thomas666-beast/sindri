package requests

import (

	"github.com/thomas666-beast/reqval"
)

type CreateWarning struct {
	reqval.Base

	UserID   string `json:"user_id"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

func (r *CreateWarning) Rules() map[string][]string {
	return map[string][]string{
		"UserID":   {"required", "uuid"},
		"Reason":   {"required", "min:1", "max:2000"},
		"Severity": {"required", "in:info,warning,severe"},
	}
}
