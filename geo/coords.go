package geo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/gml"
)

// ---- Coordinate parsing -----------------------------------------------------

// posListToLatLon returns [][lat,lon] (NOT swapped — internal use only).
func posListToLatLon(s string) ([][2]float64, error) {
	fields := strings.Fields(s)
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("geojson: posList has odd number of values")
	}
	pts := make([][2]float64, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		lat, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, err
		}
		lon, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, err
		}
		pts = append(pts, [2]float64{lat, lon})
	}
	return pts, nil
}

func posSliceToLatLon(pos []gml.DirectPosition) ([][2]float64, error) {
	pts := make([][2]float64, 0, len(pos))
	for _, p := range pos {
		lat, lon, err := firstLatLon(p.Value)
		if err != nil {
			return nil, err
		}
		pts = append(pts, [2]float64{lat, lon})
	}
	return pts, nil
}

// posListToCoords parses a GML posList (lat lon lat lon ...) into [lon,lat] pairs.
func posListToCoords(s string) ([][2]float64, error) {
	fields := strings.Fields(s)
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("geojson: posList has odd number of values")
	}
	coords := make([][2]float64, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		lat, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lat %q: %w", fields[i], err)
		}
		lon, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lon %q: %w", fields[i+1], err)
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	return coords, nil
}

// posSliceToCoords converts a slice of DirectPositionType (lat lon) to [lon,lat] pairs.
func posSliceToCoords(pos []gml.DirectPosition) ([][2]float64, error) {
	coords := make([][2]float64, 0, len(pos))
	for _, p := range pos {
		lat, lon, err := firstLatLon(p.Value)
		if err != nil {
			return nil, err
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	return coords, nil
}
