package users

type PublicUser struct {
	ID        string         `json:"id"`
	Email     string         `json:"email"`
	Username  string         `json:"username"`
	AvatarURL *string        `json:"avatar_url"`
	Socials   map[string]any `json:"socials"`
	Role      string         `json:"role"`
	CreatedAt string         `json:"created_at"`
}

func (u *User) ToPublic() PublicUser {
	pub := PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		AvatarURL: u.AvatarURL,
		Socials:   u.Socials,
		Role:      u.Role,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	return pub
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin"`
}