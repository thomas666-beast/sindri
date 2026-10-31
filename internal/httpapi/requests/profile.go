package requests

import (
	"github.com/thomas666-beast/reqval"
)

type UpdateProfile struct {
	reqval.Base

	DisplayName string   `json:"display_name"`
	Bio         string   `json:"bio"`
	Skills      []string `json:"skills"`
	Interests   []string `json:"interests"`
	AvatarURL   string   `json:"avatar_url"`
}

func (r *UpdateProfile) Rules() map[string][]string {
	return map[string][]string{
		"DisplayName": {"required", "min:1", "max:100"},
		"Bio":         {"max:2000"},
		"Skills":      {"max:30"},
		"Interests":   {"max:30"},
	}
}
