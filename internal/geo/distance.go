// Package geo provides geodesic helpers for turning lat/lon/elevation samples
// into distances.
package geo

import "math"

// earthRadiusM is the mean Earth radius used for the haversine formula.
const earthRadiusM = 6371008.8

// Haversine returns the great-circle distance in meters between two WGS84
// coordinates. Accuracy is more than sufficient for activity tracks, where
// consecutive samples are at most a few tens of meters apart.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	φ1 := rad(lat1)
	φ2 := rad(lat2)
	dφ := rad(lat2 - lat1)
	dλ := rad(lon2 - lon1)

	a := math.Sin(dφ/2)*math.Sin(dφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// Distance3D augments the horizontal great-circle distance with the vertical
// component between the two elevations (in meters). It reduces to Haversine
// when the elevation delta is zero.
func Distance3D(lat1, lon1, ele1, lat2, lon2, ele2 float64) float64 {
	flat := Haversine(lat1, lon1, lat2, lon2)
	de := ele2 - ele1
	return math.Sqrt(flat*flat + de*de)
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }
