package database

// CoverPhotoCoalesce selects the first photo's thumbnail, falling back to
// the full-size URL and then to empty when the pin has no photos at all
// (the lateral join then yields NULLs).
const CoverPhotoCoalesce = `COALESCE(pp.thumbnail_url, pp.photo_url, '')`

// CoverPhotoLateral joins a pin's first photo by position. Every pin-list
// query shares it: pins list/search/by-user, collections pins, favorites
// and the social feed. Left alone because they differ: the trending
// comment-count lateral (extra aggregate + score) and the collection-list
// scalar subquery (first photo across the whole collection, not the pin).
const CoverPhotoLateral = `LEFT JOIN LATERAL (
	SELECT photo_url, thumbnail_url FROM pin_photos
	WHERE pin_id = p.id
	ORDER BY position
	LIMIT 1
) pp ON true`
