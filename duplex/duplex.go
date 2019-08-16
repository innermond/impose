package duplex

func Reflow(in []int, weld, col, row int, reversed, flip bool) (out []int, err error) {
	// weld is length for elements grouping
	// assume every 2th group elements is subject duplex

	// peak elements on sheet
	peak := col * row

	// pad with zeroes (representing empty elements) to balance in elements
	lenin := len(in)
	len0 := lenin % peak
	if len0 != 0 {
		len0 = col - len0
	}

padding:
	for {
		// padding sheet
		for i := 0; i < len0; i++ {
			in = append(in, 0)
		}
		lenin = len(in)
		// we have added enough duplex ?
		if (lenin/peak)%2 == 0 {
			break padding
		}
		// fill with zeroes
		len0 = peak
	}

	colweld := col / weld
	natural, duplex := []int{}, [][]int{}
	// colect duplex groups using weld len
	for i := 0; i < lenin; i += 2 * weld {

		natural = append(natural, in[i:i+weld]...)
		duplex = append(duplex, in[i+weld:i+2*weld])
		if len(duplex)*weld < peak {
			continue
		}
		if reversed {
			rev := [][]int{}
			for i := len(duplex) - 1; i >= 0; i-- {
				rev = append(rev, duplex[i])
			}
			duplex = rev
		}
		if flip {
			flipped := [][]int{}
			for i := colweld - 1; i >= 0; i-- {
				flipped = append(flipped, duplex[i])
			}
			duplex = flipped
		}

		// flattening duplex
		flat := []int{}
		for i := 0; i < len(duplex); i++ {
			flat = append(flat, duplex[i]...)
		}

		out = append(out, natural...)
		out = append(out, flat...)

		natural, duplex = []int{}, [][]int{}
	}
	return
}
