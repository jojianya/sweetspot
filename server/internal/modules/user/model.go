package users

import "time"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
	RoleOwner = "owner"
)

type User struct {
	ID                 string         `json:"id"`
	Email              string         `json:"email"`
	PasswordHash       string         `json:"-"`
	Username           string         `json:"username"`
	AvatarURL          *string        `json:"avatar_url"`
	Socials            map[string]any `json:"socials"`
	Role               string         `json:"role"`
	SessionsValidAfter *time.Time     `json:"-"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}
