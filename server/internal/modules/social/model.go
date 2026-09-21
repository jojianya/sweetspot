package social

// Stats is the public per-user engagement summary.
type Stats struct {
	Followers   int  `json:"followers"`
	Following   int  `json:"following"`
	PinsCount   int  `json:"pins_count"`
	IsFollowing bool `json:"is_following"`
}
