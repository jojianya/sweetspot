package pins

const maxPhotosPerPin = 5

type CreatePinRequest struct {
	Lat        float64  `json:"lat" binding:"required"`
	Lng        float64  `json:"lng" binding:"required"`
	Caption    *string  `json:"caption"`
	CategoryID int      `json:"category_id" binding:"required"`
	PhotoURLs  []string `json:"photo_urls" binding:"required,min=1"`
}
