package requests

import (

	"github.com/thomas666-beast/reqval"
)

type SetAdmin struct {
	reqval.Base

	IsAdmin *bool `json:"is_admin"`
}

func (r *SetAdmin) Rules() map[string][]string {
	return map[string][]string{
		"IsAdmin": {"required"},
	}
}
