package pins

type NewPin struct {
	UserID        string
	Lat           float64
	Lng           float64
	Caption       *string
	CategoryID    int
	PhotoURLs     []string
	ThumbnailURLs []string
	Geohash       string
}

// NewPhoto is a re-encoded image ready to persist.
type NewPhoto struct {
	PhotoURL     string
	ThumbnailURL string
}

// UpdatePinPatch carries editable pin fields. A nil Photos keeps the current
// photo set; a non-nil (even empty is rejected by the handler) one replaces it.
type UpdatePinPatch struct {
	Caption    *string
	CategoryID *int
	Photos     []NewPhoto
}
