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
	return out, nil
}
