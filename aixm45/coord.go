package aixm45

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Axis is the axis of a coordinate.
type Axis int

// Coordinate axes.
const (
	Lat Axis = iota
	Long
)

func (a Axis) String() string {
	if a == Lat {
		return "latitude"
	}
	return "longitude"
}

// Format is the layout of a 4.5 coordinate string.
type Format int

// Coordinate formats (latitude shown; longitudes have three degree digits):
// "DD.dd…N", "DDMM.mm…N", "DDMMSS.ss…N", the fraction optional.
const (
	FormatD   Format = iota // decimal degrees
	FormatDM                // degrees and decimal minutes
	FormatDMS               // degrees, minutes and decimal seconds
)

func (f Format) String() string {
	return [...]string{"D", "DM", "DMS"}[f]
}

// MaxFraction is the most fraction digits a coordinate may have.
const MaxFraction = 12

// ErrCoordinate is returned for a malformed or out-of-range coordinate.
var ErrCoordinate = errors.New("aixm45: invalid coordinate")

// Coord is a parsed 4.5 coordinate: an exact value in signed decimal
// degrees (north and east positive) plus the layout of its source string.
type Coord struct {
	// Text is the trimmed source string.
	Text string
	// Axis, Format and Fraction (digits after the decimal point, 0 without
	// one) describe the source layout.
	Axis     Axis
	Format   Format
	Fraction int
	deg      *big.Rat
	south    bool // S or W, kept for zero values
}

// ParseLat parses a 4.5 latitude: "DD[.d…]", "DDMM[.d…]" or "DDMMSS[.d…]"
// followed by N or S.
func ParseLat(s string) (Coord, error) { return parse(s, Lat) }

// ParseLong parses a 4.5 longitude: "DDD[.d…]", "DDDMM[.d…]" or
// "DDDMMSS[.d…]" followed by E or W.
func ParseLong(s string) (Coord, error) { return parse(s, Long) }

func parse(s string, axis Axis) (Coord, error) {
	t := strings.TrimSpace(s)
	bad := func(why string) (Coord, error) {
		return Coord{}, fmt.Errorf("%w: %s %q: %s", ErrCoordinate, axis, s, why)
	}
	if len(t) < 2 {
		return bad("too short")
	}
	hemi := t[len(t)-1]
	neg := false
	switch {
	case axis == Lat && (hemi == 'N' || hemi == 'S'):
		neg = hemi == 'S'
	case axis == Long && (hemi == 'E' || hemi == 'W'):
		neg = hemi == 'W'
	default:
		return bad("no hemisphere letter")
	}
	body := t[:len(t)-1]
	intPart, frac, hasDot := strings.Cut(body, ".")
	if !digits(intPart) || (hasDot && !digits(frac)) {
		return bad("not a number")
	}
	if len(frac) > MaxFraction {
		return bad("too many fraction digits")
	}
	dd := 2
	if axis == Long {
		dd = 3
	}
	var f Format
	switch len(intPart) {
	case dd:
		f = FormatD
	case dd + 2:
		f = FormatDM
	case dd + 4:
		f = FormatDMS
	default:
		return bad("unexpected number of digits")
	}
	// The last unit (degrees, minutes or seconds) carries the fraction.
	parts := []string{intPart[:dd]}
	for i := dd; i < len(intPart); i += 2 {
		parts = append(parts, intPart[i:i+2])
	}
	last, ok := new(big.Rat).SetString(parts[len(parts)-1] + "." + frac + "0")
	if !ok {
		return bad("not a number")
	}
	deg := new(big.Rat)
	div := int64(1)
	for i, p := range parts {
		var v *big.Rat
		if i == len(parts)-1 {
			v = last
		} else {
			v, _ = new(big.Rat).SetString(p)
		}
		if i > 0 && v.Cmp(big.NewRat(60, 1)) >= 0 {
			return bad("minutes or seconds not below 60")
		}
		deg.Add(deg, v.Quo(v, big.NewRat(div, 1)))
		div *= 60
	}
	limit := int64(90)
	if axis == Long {
		limit = 180
	}
	if deg.Cmp(big.NewRat(limit, 1)) > 0 {
		return bad("out of range")
	}
	if neg {
		deg.Neg(deg)
	}
	return Coord{Text: t, Axis: axis, Format: f, Fraction: len(frac), deg: deg, south: neg}, nil
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Rat returns the exact value in signed decimal degrees (a copy).
func (c Coord) Rat() *big.Rat {
	if c.deg == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Set(c.deg)
}

// Degrees returns the value in signed decimal degrees as a float64.
func (c Coord) Degrees() float64 {
	f, _ := c.Rat().Float64()
	return f
}

// Decimal returns the value in signed decimal degrees rounded (half away
// from zero) to n fraction digits, e.g. "48.70216667".
func (c Coord) Decimal(n int) string {
	s := c.Rat().FloatString(n)
	if strings.Trim(s, "-0.") == "" {
		s = strings.TrimPrefix(s, "-") // no "-0.000"
	}
	return s
}

// MinDecimals is the fewest fraction digits Decimals returns.
const MinDecimals = 8

// Decimals returns the number of fraction digits of decimal degrees that
// preserve the source precision: rounding the decimal value back to the
// source layout reproduces the source string.  It is at least MinDecimals
// (8 digits, about 1 mm).
func (c Coord) Decimals() int {
	need := c.Fraction
	switch c.Format {
	case FormatDM:
		need += 2 // a unit of the last minute digit is 1/60 of a degree unit: 10^-k/60 > 10^-(k+2)
	case FormatDMS:
		need += 4 // 10^-k/3600 > 10^-(k+4)
	}
	if need < MinDecimals {
		need = MinDecimals
	}
	return need
}

// Canonical returns the canonical decimal-degree form used in natural keys:
// the exact value rounded to 10 fraction digits, trailing zeros removed
// ("48.7021666667", "2", "-0.5").  Equal positions written in different
// layouts (484207.80N, 484207.8N, 4842.13N) give the same string.
func (c Coord) Canonical() string {
	s := c.Decimal(10)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

// Layout renders the coordinate in a 4.5 layout with frac fraction digits,
// rounding half away from zero (the inverse of ParseLat/ParseLong).
func (c Coord) Layout(f Format, frac int) string {
	v := c.Rat()
	neg := v.Sign() < 0 || (v.Sign() == 0 && c.south)
	v.Abs(v)
	hemi := "N"
	switch {
	case c.Axis == Lat && neg:
		hemi = "S"
	case c.Axis == Long && neg:
		hemi = "W"
	case c.Axis == Long:
		hemi = "E"
	}
	dd := 2
	if c.Axis == Long {
		dd = 3
	}
	// Scale to the last unit, round, then split into units.
	unit := map[Format]int64{FormatD: 1, FormatDM: 60, FormatDMS: 3600}[f]
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(frac)), nil)
	x := new(big.Rat).Mul(v, new(big.Rat).SetInt(new(big.Int).Mul(scale, big.NewInt(unit))))
	n := roundHalfUp(x) // in units of 10^-frac of the last unit
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(n, scale, r) // q: whole last units, r: fraction
	var b strings.Builder
	switch f {
	case FormatD:
		fmt.Fprintf(&b, "%0*d", dd, q)
	case FormatDM:
		d, m := new(big.Int), new(big.Int)
		d.QuoRem(q, big.NewInt(60), m)
		fmt.Fprintf(&b, "%0*d%02d", dd, d, m)
	case FormatDMS:
		d, rest, m, sec := new(big.Int), new(big.Int), new(big.Int), new(big.Int)
		d.QuoRem(q, big.NewInt(3600), rest)
		m.QuoRem(rest, big.NewInt(60), sec)
		fmt.Fprintf(&b, "%0*d%02d%02d", dd, d, m, sec)
	}
	if frac > 0 {
		fmt.Fprintf(&b, ".%0*d", frac, r)
	}
	return b.String() + hemi
}

// roundHalfUp rounds a non-negative rational to the nearest integer, halves
// up.
func roundHalfUp(x *big.Rat) *big.Int {
	n := new(big.Int).Mul(x.Num(), big.NewInt(2))
	n.Add(n, x.Denom())
	d := new(big.Int).Mul(x.Denom(), big.NewInt(2))
	return n.Quo(n, d)
}
