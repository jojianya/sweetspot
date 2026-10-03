package geohash

import "github.com/mmcloughlin/geohash"

const defaultPrecision = 7

func Encode(lat, lng float64) string {
	return geohash.EncodeWithPrecision(lat, lng, defaultPrecision)
}
