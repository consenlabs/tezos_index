package utils

import "sort"

func Max64(x, y int64) int64 {
	if x < y {
		return y
	}
	return x
}

func NonZeroMin64(x ...int64) int64 {
	var min int64
	for _, v := range x {
		if v != 0 {
			if min == 0 {
				min = v
			} else {
				min = Min64(min, v)
			}
		}
	}
	return min
}

func Min64(x, y int64) int64 {
	if x > y {
		return y
	}
	return x
}

func Max64N(nums ...int64) int64 {
	switch len(nums) {
	case 0:
		return 0
	case 1:
		return nums[0]
	default:
		n := nums[0]
		for _, v := range nums[1:] {
			if v > n {
				n = v
			}
		}
		return n
	}
}

func Max(x, y int) int {
	if x < y {
		return y
	} else {
		return x
	}
}

type Uint64Sorter []uint64

func (s Uint64Sorter) Sort() {
	if !sort.IsSorted(s) {
		sort.Sort(s)
	}
}

func (s Uint64Sorter) Len() int           { return len(s) }
func (s Uint64Sorter) Less(i, j int) bool { return s[i] < s[j] }
func (s Uint64Sorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func UniqueUint64Slice(a []uint64) []uint64 {
	if len(a) == 0 {
		return a
	}
	b := make([]uint64, len(a))
	copy(b, a)
	Uint64Sorter(b).Sort()
	j := 0
	for i := 1; i < len(b); i++ {
		if b[j] == b[i] {
			continue
		}
		j++
		b[j] = b[i]
	}
	return b[:j+1]
}
