package geohash

import "github.com/mmcloughlin/geohash"

const defaultPrecision = 7

func Encode(lat, lng float64) string {
	return geohash.EncodeWithPrecision(lat, lng, defaultPrecision)
}

func Neighbors(lat, lng float64) []string {
	code := Encode(lat, lng)

	neighbors := geohash.Neighbors(code)
	all := make([]string, 0, len(neighbors)+1)
	all = append(all, code)
	for _, n := range neighbors {
		all = append(all, n)
	}
	return all
}
