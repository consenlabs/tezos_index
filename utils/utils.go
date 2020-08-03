package utils

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
