package duplex

import "fmt"

func Reflow(in []int, weld, col, row int, reversed, flip bool) (out []int, err error) {
	// weld is length for elements grouping
	// assume every 2th group elements is subject duplex

	// peak elements on sheet
	peak := col * row

	// pad with zeroes (representing empty elements) to balance in elements
	lenin := len(in)
	len0 := lenin % peak
	if len0 != 0 {
    // difference to fill a page
		len0 = peak - len0
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

	if weld > col || col%weld != 0 {
		err = fmt.Errorf("unsynced weld %d with col %d; weld must divide col", weld, col)
		return
	}

	colweld := col / weld
	natural, duplex := []int{}, [][]int{}
	// colect duplex groups using weld len
	for i := 0; i < lenin; i += 2 * weld {

		natural = append(natural, in[i:i+weld]...)
		duplex = append(duplex, in[i+weld:i+2*weld])
		// is this cycle at end?
		if len(duplex)*weld < peak {
			continue
		}
		// process results of cycle
		if reversed {
			rev := [][]int{}
			for i := len(duplex) - 1; i >= 0; i-- {
				// reverse duplex[i]
				revdup := []int{}
				for j := len(duplex[i]) - 1; j >= 0; j-- {
					revdup = append(revdup, duplex[i][j])
				}
				duplex[i] = revdup
				rev = append(rev, duplex[i])
			}
			duplex = rev
		}
		if flip {
			flipped := [][]int{}
			for len(duplex) > 0 {
				for i := colweld - 1; i >= 0; i-- {
					flipped = append(flipped, duplex[i])
				}
				duplex = duplex[colweld:]
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

		// start a new cycle
		natural, duplex = []int{}, [][]int{}
	}
	return
}
