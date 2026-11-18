package geo_test

import (
	"math"
	"testing"

	"github.com/davidkohl/goaixm/geo"
)

// Flinders Peak -> Buninyong, the classic Vincenty (1975) test line.
// The expected distance was cross-checked against an independent
// implementation of the Vincenty inverse solution.
const (
	flindersLat, flindersLon   = -37.95103333333333, 144.42486888888888
	buninyongLat, buninyongLon = -37.65282305555555, 143.92649583333333
	flindersBuninyongMetres    = 54972.185880
)

func TestGeodesicDistanceKnownValue(t *testing.T) {
	got := geo.GeodesicDistance(flindersLat, flindersLon, buninyongLat, buninyongLon)
	if diff := math.Abs(got - flindersBuninyongMetres); diff > 0.001 {
		t.Errorf("GeodesicDistance = %.6f m, want %.6f m (diff %.6f)",
			got, flindersBuninyongMetres, diff)
	}
}

func TestGeodesicDistanceCoincident(t *testing.T) {
	if d := geo.GeodesicDistance(52.5, 13.4, 52.5, 13.4); d != 0 {
		t.Errorf("distance between coincident points = %v, want 0", d)
	}
}

// TestDestinationRoundTrip checks the direct solution against the inverse one.
// These are independent code paths, so agreement is meaningful.
func TestDestinationRoundTrip(t *testing.T) {
	var worst float64
	for _, lat := range []float64{0, 15, 45, 52.5, 60, 75, 89} {
		for _, bearing := range []float64{0, 37, 90, 180, 270, 359} {
			for _, dist := range []float64{100, 1852, 27780, 92600, 500000} {
				destLat, destLon := geo.DestinationPoint(lat, 10, bearing, dist)
				back := geo.GeodesicDistance(lat, 10, destLat, destLon)
				if e := math.Abs(back - dist); e > worst {
					worst = e
				}
			}
		}
	}
	if worst > 1e-5 {
		t.Errorf("worst direct/inverse round-trip error %.9g m, want <= 1e-5", worst)
	}
}

func TestDestinationPointZeroDistance(t *testing.T) {
	lat, lon := geo.DestinationPoint(52.5, 13.4, 123, 0)
	if lat != 52.5 || lon != 13.4 {
		t.Errorf("zero-distance destination = (%v,%v), want (52.5,13.4)", lat, lon)
	}
}

// TestDestinationPointDueNorth is a sanity check with an analytically obvious
// answer: travelling due north must hold longitude constant and increase
// latitude.
func TestDestinationPointDueNorth(t *testing.T) {
	lat, lon := geo.DestinationPoint(50, 8, 0, 111000)
	if math.Abs(lon-8) > 1e-9 {
		t.Errorf("due-north travel changed longitude: %v", lon)
	}
	if lat <= 50 {
		t.Errorf("due-north travel did not increase latitude: %v", lat)
	}
}

func TestLengthToMetres(t *testing.T) {
	tests := []struct {
		value float64
		uom   string
		want  float64
	}{
		{15, "NM", 27780},
		{15, "[nmi_i]", 27780}, // UCUM spelling
		{10, "nm", 18520},
		{20, "km", 20000},
		{20, "KM", 20000},
		{5, "M", 5},
		{5, "m", 5},
		{1000, "FT", 304.8},
		{1000, "[ft_i]", 304.8},
		{1, "MI", 1609.344},
		{100, "CM", 1},
		{15, "  NM  ", 27780}, // surrounding whitespace tolerated
	}
	for _, tc := range tests {
		got, err := geo.LengthToMetres(tc.value, tc.uom)
		if err != nil {
			t.Errorf("LengthToMetres(%v, %q): unexpected error %v", tc.value, tc.uom, err)
			continue
		}
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("LengthToMetres(%v, %q) = %v, want %v", tc.value, tc.uom, got, tc.want)
		}
	}
}

// TestLengthToMetresRejectsUnknown pins the deliberate choice to fail rather
// than default: treating an unknown unit as metres would silently shrink a
// nautical-mile radius by a factor of 1852.
func TestLengthToMetresRejectsUnknown(t *testing.T) {
	for _, uom := range []string{"", "FURLONGS", "deg", "urn:ogc:def:uom:EPSG::9001"} {
		if _, err := geo.LengthToMetres(1, uom); err == nil {
			t.Errorf("LengthToMetres(1, %q): expected an error, got nil", uom)
		}
	}
}
