package geo

import (
	"math"
	"runtime"
	"testing"
)

// TestDetMathMatchesMath: on amd64 (no fused multiply-add) the guarded
// copies give exactly the math package's bits; elsewhere the math package
// may differ (that is why the copies exist).
func TestDetMathMatchesMath(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("the math package fuses multiply-adds on", runtime.GOARCH)
	}
	bad := map[string]int{}
	for i := 0; i < 200000; i++ {
		x := (float64(i) - 100000) * 0.000137
		if dsin(x) != math.Sin(x) {
			bad["sin"]++
		}
		if dcos(x) != math.Cos(x) {
			bad["cos"]++
		}
		s, c := dsincos(x)
		if s2, c2 := math.Sincos(x); s != s2 || c != c2 {
			bad["sincos"]++
		}
		if dtan(x) != math.Tan(x) {
			bad["tan"]++
		}
		if datan(x) != math.Atan(x) {
			bad["atan"]++
		}
		if datan2(x, 0.3) != math.Atan2(x, 0.3) || datan2(-0.3, x) != math.Atan2(-0.3, x) {
			bad["atan2"]++
		}
		if y := x / 14; dasin(y) != math.Asin(y) || dacos(y) != math.Acos(y) {
			bad["asin/acos"]++
		}
		if dhypot(x, 1.7) != math.Hypot(x, 1.7) {
			bad["hypot"]++
		}
	}
	if len(bad) > 0 {
		t.Errorf("differences from the math package: %v", bad)
	}
}
