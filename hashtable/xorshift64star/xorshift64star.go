package xorshift64star

type XorShift64Star struct {
	s uint64
}

// Seed is a non-zero value
func New(seed int64) *XorShift64Star {
	x := new(XorShift64Star)
	x.s = uint64(seed)
	return x
}

func (x *XorShift64Star) Next() uint64 {
	r := x.s * uint64(2685821657736338717)
	x.s ^= x.s >> 12
	x.s ^= x.s << 25
	x.s ^= x.s >> 27

	return r
}
