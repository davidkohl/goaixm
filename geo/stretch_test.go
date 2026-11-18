package geo

import (
	"math"
	"testing"
)

func sameCoords(t *testing.T, what string, got, want [][2]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %v, want %v", what, got, want)
	}
	for i := range got {
		if math.Abs(got[i][0]-want[i][0]) > 1e-12 || math.Abs(got[i][1]-want[i][1]) > 1e-12 {
			t.Fatalf("%s: position %d %v, want %v", what, i, got[i], want[i])
		}
	}
}

// A fictional open border: a zigzag along 11°N.
var zigzag = [][2]float64{{20, 11}, {20.25, 11.05}, {20.5, 10.95}, {20.75, 11.05}, {21, 11}}

func TestBorderStretchOpen(t *testing.T) {
	// Between two border vertices: the vertex strictly between.
	s, err := BorderStretch(zigzag, zigzag[1], zigzag[3], StretchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sameCoords(t, "forward", s.Coords, zigzag[1:4])
	if s.Reversed || s.Closed || s.Wrapped || s.FromOffset != 0 || s.ToOffset != 0 {
		t.Errorf("forward: %+v", s)
	}
	// Against the border's direction.
	s, _ = BorderStretch(zigzag, zigzag[3], zigzag[1], StretchOptions{})
	sameCoords(t, "reversed", s.Coords, [][2]float64{zigzag[3], zigzag[2], zigzag[1]})
	if !s.Reversed {
		t.Error("not reversed")
	}
	// Ends off the border are kept; the offset is their distance to the
	// border.  A point 0.01° south of the zigzag's southern tip: its
	// nearest border point is the vertex itself.
	from := [2]float64{20.5, 10.94}
	s, _ = BorderStretch(zigzag, from, zigzag[4], StretchOptions{})
	sameCoords(t, "off border", s.Coords, [][2]float64{from, zigzag[3], zigzag[4]})
	want := GeodesicDistance(10.94, 20.5, 10.95, 20.5)
	if math.Abs(s.FromOffset-want) > 1 {
		t.Errorf("offset %.1f m, want about %.1f m", s.FromOffset, want)
	}
	// With cut points the stretch joins the border at the projection.
	s, _ = BorderStretch(zigzag, from, zigzag[4], StretchOptions{CutPoints: true})
	if len(s.Coords) != 4 || s.Coords[1] != zigzag[2] {
		t.Errorf("cut points: %v", s.Coords)
	}
	// Wrap has no effect on an open border.
	w, _ := BorderStretch(zigzag, zigzag[1], zigzag[3], StretchOptions{Wrap: true})
	sameCoords(t, "open wrap", w.Coords, zigzag[1:4])
	if _, err := BorderStretch(zigzag[:1], zigzag[0], zigzag[0], StretchOptions{}); err != ErrShortBorder {
		t.Errorf("short border: %v", err)
	}
}

func TestBorderStretchClosed(t *testing.T) {
	// A fictional closed square border, counter-clockwise from its
	// south-west corner.
	sq := [][2]float64{{20.4, 11.9}, {20.6, 11.9}, {20.6, 12.1}, {20.4, 12.1}, {20.4, 11.9}}
	from, to := [2]float64{20.4333, 11.9}, [2]float64{20.4, 12.0}
	// Without Wrap: the way that does not pass the closing corner (long).
	s, _ := BorderStretch(sq, from, to, StretchOptions{})
	sameCoords(t, "no wrap", s.Coords, [][2]float64{from, sq[1], sq[2], sq[3], to})
	if !s.Closed || s.Wrapped {
		t.Errorf("no wrap: %+v", s)
	}
	// With Wrap: the shorter way, through the closing corner.
	s, _ = BorderStretch(sq, from, to, StretchOptions{Wrap: true})
	sameCoords(t, "wrap", s.Coords, [][2]float64{from, sq[0], to})
	if !s.Closed || !s.Wrapped {
		t.Errorf("wrap: %+v", s)
	}
	// The other direction gives the same stretch reversed.
	r, _ := BorderStretch(sq, to, from, StretchOptions{Wrap: true})
	sameCoords(t, "wrap reversed", r.Coords, [][2]float64{to, sq[0], from})
	// When the direct way is shorter, it is kept.
	s, _ = BorderStretch(sq, [2]float64{20.45, 11.9}, [2]float64{20.55, 11.9}, StretchOptions{Wrap: true})
	if s.Wrapped || len(s.Coords) != 2 {
		t.Errorf("direct: %+v", s)
	}
}

// TestBorderStretchIsTrim: BorderStretch cuts like a referenced border
// between explicit neighbours in a ring.
func TestBorderStretchIsTrim(t *testing.T) {
	from, to := [2]float64{20.1, 10.99}, [2]float64{20.9, 11.04}
	members := []ringMember{
		{coords: [][2]float64{{20.9, 10.5}, {20.1, 10.5}, from}},
		{coords: zigzag, isBorder: true, href: "b"},
		{coords: [][2]float64{to, {20.9, 10.5}}},
	}
	trimBorders(members, false)
	s, _ := BorderStretch(zigzag, from, to, StretchOptions{})
	sameCoords(t, "trim", s.Coords, members[1].coords)
}

func FuzzBorderStretch(f *testing.F) {
	f.Add(20.1, 10.99, 20.9, 11.04, true)
	f.Add(20.4, 11.0, 20.4, 11.0, false)
	f.Fuzz(func(t *testing.T, x1, y1, x2, y2 float64, wrap bool) {
		for _, v := range []float64{x1, y1, x2, y2} {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1000 {
				return
			}
		}
		from, to := [2]float64{x1, y1}, [2]float64{x2, y2}
		for _, b := range [][][2]float64{zigzag, {{20.4, 11.9}, {20.6, 11.9}, {20.6, 12.1}, {20.4, 12.1}, {20.4, 11.9}}} {
			s, err := BorderStretch(b, from, to, StretchOptions{Wrap: wrap, CutPoints: !wrap})
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Coords) < 1 || s.Coords[0] != from || s.Coords[len(s.Coords)-1] != to && !samePoint(s.Coords[len(s.Coords)-1], to) {
				t.Fatalf("ends of %v: %v", b, s.Coords)
			}
		}
	})
}
