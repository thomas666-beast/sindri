package requests

import (
	"context"

	"github.com/thomas666-beast/reqval"
)

type Register struct {
	reqval.Base

	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *Register) Rules() map[string][]string {
	return map[string][]string{
		"Email":    {"required", "email"},
		"Username": {"required", "min:3", "max:32", "regexp:^[a-z0-9_]+$"},
		"Password": {"required", "min:8", "max:72"},
	}
}

func (r *Register) Messages() map[string]string {
	return map[string]string{
		"Username.regexp": "username must contain only lowercase letters, digits, or underscores",
	}
}

func (r *Register) Authorize(ctx context.Context) error {
	return nil
}
