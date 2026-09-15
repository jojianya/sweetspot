package users

type PublicUser struct {
	ID        string         `json:"id"`
	Username  string         `json:"username"`
	AvatarURL *string        `json:"avatar_url"`
	Socials   map[string]any `json:"socials"`
	Role      string         `json:"role"`
	CreatedAt string         `json:"created_at"`
}

func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Username:  u.Username,
		AvatarURL: u.AvatarURL,
		Socials:   u.Socials,
		Role:      u.Role,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// PrivateUser is the shape returned to the account owner themselves: the
// public profile plus the verified email address.
type PrivateUser struct {
	PublicUser
	Email string `json:"email"`
}

func (u *User) ToPrivate() PrivateUser {
	return PrivateUser{
		PublicUser: u.ToPublic(),
		Email:      u.Email,
	}
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin owner"`
}
