package booklet

import "fmt"

func Arrange(col, row int, pp []int) (out []int, err error) {
	if col%2 != 0 {
		err = fmt.Errorf("column number %d must be even", col)
		return
	}
	np := len(pp)
	if np%(col*row) != 0 {
		err = fmt.Errorf("number of pages %d is not proper for grid %s, try with only %d pages or add more %d pages",
			np, fmt.Sprintf("%dx%d", col, row),
			np-np%(col*row),
			np%(col*row),
		)
		return
	}

	a := 0
	i, j, k, prev := 0, 0, 0, 0
	z := len(pp) - 1
	lenp := len(pp) / 2
	back := false
	sign := 1
	pair := [2]int{z, a}
	num := col * row
	var pairs [][2]int
	pairs = append(pairs, pair)
	for {
		if j == lenp {
			break
		}
		if a+1 == z {
			break
		}
		i += 2
		k = (j + num/2) % (num / 2)
		prev = j - k
		if i == num {
			i = 0
			a = pairs[prev][1] + sign
			z = pairs[prev][0] - sign
			if back {
				a, z = z, a
			}
			back = !back
			sign = -1 * sign
		} else {
			prev = len(pairs) - 1
			a = pairs[prev][1] + 2*sign
			z = pairs[prev][0] - 2*sign
			if back {
				a, z = z, a
			}
		}
		if back {
			pair = [2]int{a, z}
		} else {
			pair = [2]int{z, a}
		}
		pairs = append(pairs, pair)
		j++
	}

	for _, pair := range pairs {
		out = append(out, pp[pair[0]], pp[pair[1]])
	}

	if col > 2 {
		reout := []int{}
		for i, j := 0, 2; i < np; i, j = i+num, j+1 {
			if j%2 == 0 {
				reout = append(reout, out[i:i+num]...)
				continue
			}
			// reverse by 2 elem welded
			src := append([]int{}, out[i:i+num]...)
			welded := [][2]int{}
			for i := 0; i < len(src); i += 2 {
				welded = append(welded, [2]int{src[i], src[i+1]})
			}
			// reverse welded
			rev := [][2]int{}
			for j := len(welded) - 1; j >= 0; j-- {
				rev = append(rev, welded[j])
			}
			// flat reversed
			for i := 0; i < len(rev); i++ {
				reout = append(reout, rev[i][0], rev[i][1])
			}
			out = reout
		}
	}

	return out, nil
}
