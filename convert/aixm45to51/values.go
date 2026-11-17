package aixm45to51

import (
	"math/big"
	"regexp"
	"unicode/utf8"
)

// textType is a 5.1.1 string type: length (in characters) and pattern.
type textType struct {
	name     string
	min, max int
	re       *regexp.Regexp
}

var (
	reChar1 = regexp.MustCompile(`^([A-Z]|[0-9])+([ +\-/]*([A-Z]|[0-9])+)*$`)
	reChar3 = regexp.MustCompile(`^([A-Z]|[0-9]|[, !"&#$%'()*+\-./:;<=>?@\[\\\]^_|{}])*$`)
	reAlnum = regexp.MustCompile(`^([A-Z]|[0-9])*$`)
	reAlpha = regexp.MustCompile(`^[A-Z]*$`)
	reChan  = regexp.MustCompile(`^[0-9]{1,3}[WXYZ]$`)
)

var (
	ttName      = textType{"TextNameType", 1, 60, reChar3}
	ttDesig     = textType{"TextDesignatorType", 1, 16, reChar3}
	ttDpnDesig  = textType{"CodeDesignatedPointDesignatorType", 1, 5, reAlnum}
	ttNavDesig  = textType{"CodeNavaidDesignatorType", 1, 4, reAlnum}
	ttAhpDesig  = textType{"CodeAirportHeliportDesignatorType", 3, 6, reAlnum}
	ttOrgDesig  = textType{"CodeOrganisationDesignatorType", 1, 12, reChar1}
	ttICAO      = textType{"CodeICAOType", 4, 4, reAlpha}
	ttIATA      = textType{"CodeIATAType", 3, 3, reAlpha}
	ttNote      = textType{"TextNoteType", 1, 10000, nil}
	ttChannel   = textType{"CodeDMEChannelType", 2, 4, reChan}
	ttTACANChan = textType{"CodeTACANChannelType", 2, 4, reChan}
)

func (t textType) valid(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= t.min && n <= t.max && (t.re == nil || t.re.MatchString(s))
}

// numType is a 5.1.1 numeric type: pattern of the lexical form and range.
type numType struct {
	name     string
	re       *regexp.Regexp
	min, max *big.Rat // nil: unbounded
	special  []string // accepted non-numeric values
	minExcl  bool     // min is exclusive
}

var (
	reDecimal = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)$`)
	reDistVer = regexp.MustCompile(`^[+-]?[0-9]{1,8}(\.[0-9]{1,4})?$`)
	reYear    = regexp.MustCompile(`^[1-9][0-9]{3}$`)
	rePCN     = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9])?$`)
)

func rat(n int64) *big.Rat { return big.NewRat(n, 1) }

var (
	ntDistance  = numType{"ValDistanceType", reDecimal, rat(0), nil, nil, false}
	ntSigned    = numType{"ValDistanceSignedType", reDecimal, nil, nil, nil, false}
	ntVertical  = numType{"ValDistanceVerticalType", reDistVer, nil, nil, []string{"UNL", "GND", "FLOOR", "CEILING"}, false}
	ntFrequency = numType{"ValFrequencyType", reDecimal, rat(0), nil, nil, true}
	ntBearing   = numType{"ValBearingType", reDecimal, rat(0), rat(360), nil, false}
	ntMagVar    = numType{"ValMagneticVariationType", reDecimal, rat(-180), rat(180), nil, false}
	ntTemp      = numType{"ValTemperatureType", reDecimal, nil, nil, nil, false}
	ntYear      = numType{"DateYearType", reYear, nil, nil, nil, false}
	ntPCN       = numType{"ValPCNType", rePCN, nil, nil, nil, false}
)

func (t numType) valid(s string) bool {
	for _, sp := range t.special {
		if s == sp {
			return true
		}
	}
	if !t.re.MatchString(s) {
		return false
	}
	if t.min == nil && t.max == nil {
		return true
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok {
		return false
	}
	lo := 0
	if t.minExcl {
		lo = 1
	}
	return (t.min == nil || v.Cmp(t.min) >= lo) && (t.max == nil || v.Cmp(t.max) <= 0)
}
