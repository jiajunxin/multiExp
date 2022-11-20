package multiexp

import (
	"fmt"
	. "math/big"
)

type PreTable struct {
	Base      *Int
	Modulos   *Int
	TableSize int
	table     [][]nat
}

func PreCompute(base, modular *Int, tableSize int) *PreTable {
	if base == nil || modular == nil {
		return nil
	}
	if base.Sign() <= 0 || modular.Sign() <= 0 {
		return nil
	}
	x := base.Bits()
	if len(x) == 0 {
		return nil
	}

	if len(x) == 1 && x[0] == 1 {
		return nil
	}

	// x > 1

	m := modular.Bits() // m.abs may be nil for m == 0
	numWords := len(m)
	if numWords == 0 {
		return nil
	}

	var table PreTable
	table.Base = base
	table.Modulos = modular
	table.TableSize = tableSize
	// calculate the table
	// Ideally the precomputations would be performed outside, and reused
	// k0 = -m**-1 mod 2**_W. Algorithm from: Dumas, J.G. "On Newton–Raphson
	// Iteration for Multiplicative Inverses Modulo Prime Powers".
	k0 := 2 - m[0]
	t := m[0] - 1
	for i := 1; i < _W; i <<= 1 {
		t *= t
		k0 *= (t + 1)
	}
	k0 = -k0

	// RR = 2**(2*_W*len(m)) mod m
	RR := nat(nil).setWord(1)
	zz1 := nat(nil).shl(RR, uint(2*numWords*_W))
	_, RR = nat(nil).div(RR, zz1, m)
	if len(RR) < numWords {
		zz1 = zz1.make(numWords)
		copy(zz1, RR)
		RR = zz1
	}

	// one = 1, with equal length to that of m
	one := make(nat, numWords)
	one[0] = 1

	// powers[i] contains x^i
	var powers [2]nat
	powers[0] = powers[0].montgomery(one, RR, m, k0, numWords)
	powers[1] = powers[1].montgomery(x, RR, m, k0, numWords)
	var temp, squaredPower nat
	temp = temp.make(numWords)
	squaredPower = squaredPower.make(numWords)
	copy(squaredPower, powers[1])
	pretable := make([][]nat, tableSize)
	for i := range pretable {
		pretable[i] = make([]nat, _W)
	}

	for i := 0; i < tableSize; i++ {
		for j := 0; j < _W; j++ {
			// montgomery must have the returned value not same as the input values
			// we have to use this temp as the middle variable
			pretable[i][j] = squaredPower
			temp = temp.montgomery(squaredPower, squaredPower, m, k0, numWords)
			squaredPower, temp = temp, squaredPower
		}
	}

	table.table = pretable
	return &table
}

// fourfoldExpNNMontgomery calculates x**y1 mod m and x**y2 mod m x**y3 mod m and x**y4 mod m
// Uses Montgomery representation.
func fourfoldExpNNMontgomeryWithPreComputeTableParallel(x, m nat, y []*Int, pretable *PreTable) []*Int {
	numWords := len(m)

	// We want the lengths of x and m to be equal.
	// It is OK if x >= m as long as len(x) == len(m).
	if len(x) > numWords {
		_, x = nat(nil).div(nil, x, m)
		// Note: now len(x) <= numWords, not guaranteed ==.
	}
	if len(x) < numWords {
		rr := make(nat, numWords)
		copy(rr, x)
		x = rr
	}

	// Ideally the precomputations would be performed outside, and reused
	// k0 = -m**-1 mod 2**_W. Algorithm from: Dumas, J.G. "On Newton–Raphson
	// Iteration for Multiplicative Inverses Modulo Prime Powers".
	k0 := 2 - m[0]
	t := m[0] - 1
	for i := 1; i < _W; i <<= 1 {
		t *= t
		k0 *= (t + 1)
	}
	k0 = -k0

	// RR = 2**(2*_W*len(m)) mod m
	RR := nat(nil).setWord(1)
	zz1 := nat(nil).shl(RR, uint(2*numWords*_W))
	_, RR = nat(nil).div(RR, zz1, m)
	if len(RR) < numWords {
		zz1 = zz1.make(numWords)
		copy(zz1, RR)
		RR = zz1
	}

	// one = 1, with equal length to that of m
	one := make(nat, numWords)
	one[0] = 1

	// powers[i] contains x^i
	var powers [2]nat
	powers[0] = powers[0].montgomery(one, RR, m, k0, numWords)
	powers[1] = powers[1].montgomery(x, RR, m, k0, numWords)

	// Zero round, find common bits of the four values
	//fmt.Println("test here, len = ", len([]nat{y[0].abs, y[1].abs, y[2].abs, y[3].abs}))
	fmt.Println("test before fourfold gcb")
	StatforInt(y[0].Bits())
	StatforInt(y[1].Bits())
	StatforInt(y[2].Bits())
	StatforInt(y[3].Bits())
	yNew := fourfoldGcb([]nat{y[0].Bits(), y[1].Bits(), y[2].Bits(), y[3].Bits()})
	fmt.Println("test after fourfold gcb")
	fmt.Println("------yNew[0]--------")
	StatforInt(yNew[0])
	StatforInt(yNew[1])
	StatforInt(yNew[2])
	StatforInt(yNew[3])
	fmt.Println("test for the new common bits")
	StatforInt(yNew[4])
	// First round, find common bits of the three values
	var cm012, cm013, cm023, cm123 nat
	cm012 = threefoldGcb(yNew[:3])
	cm013 = threefoldGcb([]nat{yNew[0], yNew[1], yNew[3]})
	cm023 = threefoldGcb([]nat{yNew[0], yNew[2], yNew[3]})
	cm123 = threefoldGcb(yNew[1:4])

	var cm01, cm23, cm02, cm13, cm03, cm12 nat
	yNew[0], yNew[1], cm01 = gcb(yNew[0], yNew[1])
	yNew[2], yNew[3], cm23 = gcb(yNew[2], yNew[3])
	yNew[0], yNew[2], cm02 = gcb(yNew[0], yNew[2])
	yNew[1], yNew[3], cm13 = gcb(yNew[1], yNew[3])
	yNew[0], yNew[3], cm03 = gcb(yNew[0], yNew[3])
	yNew[1], yNew[2], cm12 = gcb(yNew[1], yNew[2])
	fmt.Println("test after the bitwise gcb")
	fmt.Println("------yNew[0]--------")
	StatforInt(yNew[0])
	StatforInt(yNew[1])
	StatforInt(yNew[2])
	StatforInt(yNew[3])
	fmt.Println("test for the new common bits")
	fmt.Println("------cm01--------")
	StatforInt(cm01)
	StatforInt(cm02)
	StatforInt(cm03)
	StatforInt(cm13)
	c1 := make(chan []nat)
	c2 := make(chan []nat)
	c3 := make(chan []nat)
	c4 := make(chan []nat)

	go multimontgomeryWithPreComputeTableWithChan(RR, m, powers[0], powers[1], k0, numWords, yNew[:2], pretable, c1)
	go multimontgomeryWithPreComputeTableWithChan(RR, m, powers[0], powers[1], k0, numWords, yNew[2:4], pretable, c2)
	go multimontgomeryWithPreComputeTableWithChan(RR, m, powers[0], powers[1], k0, numWords, []nat{yNew[4], cm012, cm013, cm023, cm123}, pretable, c3)
	go multimontgomeryWithPreComputeTableWithChan(RR, m, powers[0], powers[1], k0, numWords, []nat{cm01, cm23, cm02, cm13, cm03, cm12}, pretable, c4)

	z1 := <-c1
	z2 := <-c2
	z3 := <-c3
	z4 := <-c4
	z := append(z1, z2...)
	z = append(z, z3...)
	z = append(z, z4...)
	//                                                                    0-4	  5     6      7       8     9     10     11    12    13    14
	//z := multimontgomeryWithPreComputeTable(RR, m, powers[0], powers[1], k0, numWords, append(yNew, cm012, cm013, cm023, cm123, cm01, cm23, cm02, cm13, cm03, cm12), pretable)
	// calculate the actual values
	var temp nat
	temp = temp.make(numWords)
	// retrive common values for first number
	temp = temp.montgomery(z[0], z[4], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[5], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[6], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[7], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[9], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[11], m, k0, numWords)
	z[0], temp = temp, z[0]
	temp = temp.montgomery(z[0], z[13], m, k0, numWords)
	z[0], temp = temp, z[0]
	// retrive common values for second number
	temp = temp.montgomery(z[1], z[4], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[5], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[6], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[8], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[9], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[12], m, k0, numWords)
	z[1], temp = temp, z[1]
	temp = temp.montgomery(z[1], z[14], m, k0, numWords)
	z[1], temp = temp, z[1]
	// retrive common values for third number
	temp = temp.montgomery(z[2], z[4], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[5], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[7], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[8], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[10], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[11], m, k0, numWords)
	z[2], temp = temp, z[2]
	temp = temp.montgomery(z[2], z[14], m, k0, numWords)
	z[2], temp = temp, z[2]
	// retrive common values for four number
	temp = temp.montgomery(z[3], z[4], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[6], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[7], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[8], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[10], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[12], m, k0, numWords)
	z[3], temp = temp, z[3]
	temp = temp.montgomery(z[3], z[13], m, k0, numWords)
	z[3], temp = temp, z[3]

	z = z[:4] //the rest are useless now

	// convert to regular number
	for i := range z {
		temp = temp.montgomery(z[i], one, m, k0, numWords)
		z[i], temp = temp, z[i]
	}
	for i := range z {
		// One last reduction, just in case.
		// See golang.org/issue/13907.
		if z[i].cmp(m) >= 0 {
			// Common case is m has high bit set; in that case,
			// since zz is the same length as m, there can be just
			// one multiple of m to remove. Just subtract.
			// We think that the subtract should be sufficient in general,
			// so do that unconditionally, but double-check,
			// in case our beliefs are wrong.
			// The div is not expected to be reached.
			z[i] = z[i].sub(z[i], m)
			if z[i].cmp(m) >= 0 {
				_, z[i] = nat(nil).div(nil, z[i], m)
			}
		}
	}

	ret := make([]*Int, 4)
	for i := range ret {
		ret[i] = new(Int)
	}

	// normlize and set value
	for i := range z {
		z[i].norm()
		ret[i].SetBits(z[i])
	}
	return ret
}