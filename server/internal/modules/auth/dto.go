package auth

type RegisterRequest struct {
	Email string `json:"email" binding:"required,email"`
	// No length tag on purpose. The validator counts runes; bcrypt counts
	// bytes and rejects anything over 72. A `max=72` tag let a 72-rune
	// non-ASCII password through and then fail inside bcrypt, which surfaced as
	// a 500. Length is checked in password.Validate, which measures both bounds
	// in the unit bcrypt actually uses.
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required,min=3,max=30"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required,max=254"`
	Password   string `json:"password" binding:"required"`
}
