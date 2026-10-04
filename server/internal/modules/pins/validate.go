package pins

import (
	"math"
	"net/http"
	"strconv"
	"unicode/utf8"
)

// createPinInput carries the validated scalar fields of a create-pin form.
// Photo files and category existence stay with the caller: files need the
// multipart handle and existence needs the repository.
type createPinInput struct {
	lat        float64
	lng        float64
	categoryID int
	caption    *string
}

// validateCreateFields parses the create-pin scalar fields in handler order,
// returning the exact status and message the handler responds with.
func validateCreateFields(latStr, lngStr, categoryStr, captionStr string, photoCount int) (*createPinInput, *photoErr) {
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return nil, &photoErr{http.StatusBadRequest, "lat must be a number"}
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return nil, &photoErr{http.StatusBadRequest, "lng must be a number"}
	}
	// ParseFloat accepts "NaN"/"Inf"; NaN would slip past every range check
	// below and 500 the PostGIS insert, so reject non-finite values up front.
	if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) {
		return nil, &photoErr{http.StatusBadRequest, "latitude or longitude must be a finite number"}
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, &photoErr{http.StatusBadRequest, "latitude or longitude out of range"}
	}

	categoryID, err := strconv.Atoi(categoryStr)
	if err != nil {
		return nil, &photoErr{http.StatusBadRequest, "category_id must be an integer"}
	}

	var caption *string
	if captionStr != "" {
		if len([]rune(captionStr)) > 500 {
			return nil, &photoErr{http.StatusBadRequest, "caption must be at most 500 characters"}
		}
		caption = &captionStr
	}

	if photoCount < 1 {
		return nil, &photoErr{http.StatusBadRequest, "at least one photo is required"}
	}
	if photoCount > maxPhotosPerPin {
		return nil, &photoErr{http.StatusBadRequest, "photo count exceeds maximum"}
	}

	return &createPinInput{lat: lat, lng: lng, categoryID: categoryID, caption: caption}, nil
}

// updatePinInput carries the validated scalar fields of an update-pin form.
type updatePinInput struct {
	caption    string
	categoryID *int
}

// validateUpdateFields parses the update-pin scalar fields in handler order,
// returning the exact status and message the handler responds with. Category
// existence needs the repository and stays with the caller.
func validateUpdateFields(captionStr, categoryStr string, photoCount int) (*updatePinInput, *photoErr) {
	if utf8.RuneCountInString(captionStr) > 500 {
		return nil, &photoErr{http.StatusBadRequest, "caption must be at most 500 characters"}
	}

	var categoryID *int
	if categoryStr != "" {
		idv, err := strconv.Atoi(categoryStr)
		if err != nil {
			return nil, &photoErr{http.StatusBadRequest, "category_id must be an integer"}
		}
		categoryID = &idv
	}

	if photoCount > maxPhotosPerPin {
		return nil, &photoErr{http.StatusBadRequest, "photo count exceeds maximum"}
	}

	return &updatePinInput{caption: captionStr, categoryID: categoryID}, nil
}
