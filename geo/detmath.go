package geo

// Deterministic floating point (FMA guard).
//
// The Go specification lets a compiler fuse x*y + z into one fused
// multiply-add, which rounds once instead of twice.  The gc compiler does
// so on arm64 (and on ppc64, s390x, riscv64, and amd64 with GOAMD64=v3),
// not on amd64 with the default GOAMD64=v1.  Converted positions would
// then differ in their last bits between platforms: the same AIXM file
// gives other coordinates on an arm64 laptop than on an amd64 server.
//
// Every product that is added to or subtracted from something on the
// conversion path is therefore written float64(x*y): an explicit
// conversion rounds the product (the specification guarantees that it
// prevents fusion).  The standard library's pure-Go trigonometric
// functions are compiled the same way, so the functions below are copies
// of them with the same guard; on amd64 (v1) they give the bits of the
// math package's.  math.Sqrt is correctly rounded everywhere and is used
// as is.  TestDeterminismDigest checks the result on every architecture.
//
// The functions below are derived from the Go math package:
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license
// (https://go.dev/LICENSE): redistributions must retain this notice;
// neither the name of Google LLC nor the names of its contributors may be
// used to endorse or promote products derived from this software without
// specific prior written permission.  The original is the Cephes Math
// Library, Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier,
// whose readme says "you may use this software as you see fit".

import "math"

// reduceThreshold is the argument from which the math package reduces
// with Payne-Hanek; no angle on the conversion path comes near it, so
// such arguments are left to the math package.
const reduceThreshold = 1 << 29

var detSinP = [...]float64{
	1.58962301576546568060e-10,
	-2.50507477628578072866e-8,
	2.75573136213857245213e-6,
	-1.98412698295895385996e-4,
	8.33333333332211858878e-3,
	-1.66666666666666307295e-1,
}

var detCosP = [...]float64{
	-1.13585365213876817300e-11,
	2.08757008419747316778e-9,
	-2.75573141792967388112e-7,
	2.48015872888517045348e-5,
	-1.38888888888730564116e-3,
	4.16666666666665929218e-2,
}

const (
	pi4A = 7.85398125648498535156e-1 // Pi/4 split into three parts
	pi4B = 3.77489470793079817668e-8
	pi4C = 2.69515142907905952645e-15
)

// octant reduces x ≥ 0 (below reduceThreshold) to z in [-Pi/4, Pi/4]
// and its octant j.
func octant(x float64) (j uint64, z float64) {
	j = uint64(x * (4 / math.Pi))
	y := float64(j)
	if j&1 == 1 {
		j++
		y++
	}
	z = ((x - float64(y*pi4A)) - float64(y*pi4B)) - float64(y*pi4C)
	return j, z
}

func sinPoly(z, zz float64) float64 {
	p := float64(detSinP[0]*zz) + detSinP[1]
	p = float64(p*zz) + detSinP[2]
	p = float64(p*zz) + detSinP[3]
	p = float64(p*zz) + detSinP[4]
	p = float64(p*zz) + detSinP[5]
	return z + float64(z*zz*p)
}

func cosPoly(zz float64) float64 {
	p := float64(detCosP[0]*zz) + detCosP[1]
	p = float64(p*zz) + detCosP[2]
	p = float64(p*zz) + detCosP[3]
	p = float64(p*zz) + detCosP[4]
	p = float64(p*zz) + detCosP[5]
	return 1.0 - float64(0.5*zz) + float64(zz*zz*p)
}

// dsin is math.Sin without fused multiply-adds.
func dsin(x float64) float64 {
	switch {
	case x == 0 || math.IsNaN(x):
		return x
	case math.IsInf(x, 0):
		return math.NaN()
	}
	sign := false
	if x < 0 {
		x, sign = -x, true
	}
	if x >= reduceThreshold {
		if sign {
			return -math.Sin(x)
		}
		return math.Sin(x)
	}
	j, z := octant(x)
	j &= 7
	if j > 3 {
		sign = !sign
		j -= 4
	}
	zz := float64(z * z)
	var y float64
	if j == 1 || j == 2 {
		y = cosPoly(zz)
	} else {
		y = sinPoly(z, zz)
	}
	if sign {
		y = -y
	}
	return y
}

// dcos is math.Cos without fused multiply-adds.
func dcos(x float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsInf(x, 0):
		return math.NaN()
	}
	x = math.Abs(x)
	if x >= reduceThreshold {
		return math.Cos(x)
	}
	sign := false
	j, z := octant(x)
	j &= 7
	if j > 3 {
		j -= 4
		sign = !sign
	}
	if j > 1 {
		sign = !sign
	}
	zz := float64(z * z)
	var y float64
	if j == 1 || j == 2 {
		y = sinPoly(z, zz)
	} else {
		y = cosPoly(zz)
	}
	if sign {
		y = -y
	}
	return y
}

// dsincos is math.Sincos without fused multiply-adds.
func dsincos(x float64) (sin, cos float64) {
	switch {
	case x == 0:
		return x, 1
	case math.IsNaN(x) || math.IsInf(x, 0):
		return math.NaN(), math.NaN()
	}
	sinSign, cosSign := false, false
	if x < 0 {
		x, sinSign = -x, true
	}
	if x >= reduceThreshold {
		s, c := math.Sincos(x)
		if sinSign {
			s = -s
		}
		return s, c
	}
	j, z := octant(x)
	j &= 7
	if j > 3 {
		j -= 4
		sinSign, cosSign = !sinSign, !cosSign
	}
	if j > 1 {
		cosSign = !cosSign
	}
	zz := float64(z * z)
	cos = cosPoly(zz)
	sin = sinPoly(z, zz)
	if j == 1 || j == 2 {
		sin, cos = cos, sin
	}
	if cosSign {
		cos = -cos
	}
	if sinSign {
		sin = -sin
	}
	return sin, cos
}

var detTanP = [...]float64{
	-1.30936939181383777646e4,
	1.15351664838587416140e6,
	-1.79565251976484877988e7,
}

var detTanQ = [...]float64{
	1.00000000000000000000e0,
	1.36812963470692954678e4,
	-1.32089234440210967447e6,
	2.50083801823357915839e7,
	-5.38695755929454629881e7,
}

// dtan is math.Tan without fused multiply-adds.
func dtan(x float64) float64 {
	switch {
	case x == 0 || math.IsNaN(x):
		return x
	case math.IsInf(x, 0):
		return math.NaN()
	}
	sign := false
	if x < 0 {
		x, sign = -x, true
	}
	if x >= reduceThreshold {
		if sign {
			return -math.Tan(x)
		}
		return math.Tan(x)
	}
	j, z := octant(x)
	zz := float64(z * z)
	var y float64
	if zz > 1e-14 {
		p := float64(detTanP[0]*zz) + detTanP[1]
		p = float64(p*zz) + detTanP[2]
		q := zz + detTanQ[1]
		q = float64(q*zz) + detTanQ[2]
		q = float64(q*zz) + detTanQ[3]
		q = float64(q*zz) + detTanQ[4]
		y = z + float64(z*(zz*p/q))
	} else {
		y = z
	}
	if j&2 == 2 {
		y = -1 / y
	}
	if sign {
		y = -y
	}
	return y
}

// xatan evaluates a series valid in the range [0, 0.66].
func xatan(x float64) float64 {
	const (
		P0 = -8.750608600031904122785e-01
		P1 = -1.615753718733365076637e+01
		P2 = -7.500855792314704667340e+01
		P3 = -1.228866684490136173410e+02
		P4 = -6.485021904942025371773e+01
		Q0 = +2.485846490142306297962e+01
		Q1 = +1.650270098316988542046e+02
		Q2 = +4.328810604912902668951e+02
		Q3 = +4.853903996359136964868e+02
		Q4 = +1.945506571482613964425e+02
	)
	z := float64(x * x)
	p := float64(P0*z) + P1
	p = float64(p*z) + P2
	p = float64(p*z) + P3
	p = float64(p*z) + P4
	q := z + Q0
	q = float64(q*z) + Q1
	q = float64(q*z) + Q2
	q = float64(q*z) + Q3
	q = float64(q*z) + Q4
	z = z * p / q
	return float64(x*z) + x
}

// satan reduces its argument (known to be positive) to the range
// [0, 0.66] and calls xatan.
func satan(x float64) float64 {
	const (
		morebits = 6.123233995736765886130e-17 // pi/2 = PIO2 + morebits
		tan3pio8 = 2.41421356237309504880      // tan(3*pi/8)
	)
	if x <= 0.66 {
		return xatan(x)
	}
	if x > tan3pio8 {
		return math.Pi/2 - xatan(1/x) + morebits
	}
	return math.Pi/4 + xatan((x-1)/(x+1)) + 0.5*morebits
}

// datan is math.Atan without fused multiply-adds.
func datan(x float64) float64 {
	if x == 0 {
		return x
	}
	if x > 0 {
		return satan(x)
	}
	return -satan(-x)
}

// datan2 is math.Atan2 without fused multiply-adds.
func datan2(y, x float64) float64 {
	switch {
	case math.IsNaN(y) || math.IsNaN(x):
		return math.NaN()
	case y == 0:
		if x >= 0 && !math.Signbit(x) {
			return math.Copysign(0, y)
		}
		return math.Copysign(math.Pi, y)
	case x == 0:
		return math.Copysign(math.Pi/2, y)
	case math.IsInf(x, 0):
		if math.IsInf(x, 1) {
			if math.IsInf(y, 0) {
				return math.Copysign(math.Pi/4, y)
			}
			return math.Copysign(0, y)
		}
		if math.IsInf(y, 0) {
			return math.Copysign(3*math.Pi/4, y)
		}
		return math.Copysign(math.Pi, y)
	case math.IsInf(y, 0):
		return math.Copysign(math.Pi/2, y)
	}
	q := datan(y / x)
	if x < 0 {
		if q <= 0 {
			return q + math.Pi
		}
		return q - math.Pi
	}
	return q
}

// dasin is math.Asin without fused multiply-adds.
func dasin(x float64) float64 {
	if x == 0 {
		return x
	}
	sign := false
	if x < 0 {
		x, sign = -x, true
	}
	if x > 1 {
		return math.NaN()
	}
	temp := math.Sqrt(1 - float64(x*x))
	if x > 0.7 {
		temp = math.Pi/2 - satan(temp/x)
	} else {
		temp = satan(x / temp)
	}
	if sign {
		temp = -temp
	}
	return temp
}

// dacos is math.Acos without fused multiply-adds.
func dacos(x float64) float64 { return math.Pi/2 - dasin(x) }

// dhypot is math.Hypot without fused multiply-adds (amd64's assembly
// computes the same: max · sqrt(1 + (min/max)²), each step rounded).
func dhypot(p, q float64) float64 {
	p, q = math.Abs(p), math.Abs(q)
	switch {
	case math.IsInf(p, 1) || math.IsInf(q, 1):
		return math.Inf(1)
	case math.IsNaN(p) || math.IsNaN(q):
		return math.NaN()
	}
	if p < q {
		p, q = q, p
	}
	if p == 0 {
		return 0
	}
	q = q / p
	return p * math.Sqrt(1+float64(q*q))
}
