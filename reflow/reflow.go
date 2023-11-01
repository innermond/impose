package reflow

import (
	"errors"
	"math"
)

var (
	ErrPositiveInt = errors.New("got negative value")
)

const (
	end  = -1
	half = -2
	odd  = -3
	even = -4
)

func On(in []int, as []int) (out []int, err error) {
	l := len(as)
	// run while we have something to process
	for len(in) > 0 {

		lin := len(in)

		// init buffers
		chunk := []int{}
		in2 := []int{}

		// process in using as
		for i, x := range as {
			if i+1 > lin {
				out = append(out, chunk...)
				return
			}
			switch x {
			case end:
				chunk = append(chunk, in[lin-1])
				in2 = append(in2, lin-1)
			case half:
				var middle int
				if lin%2 == 0 {
					middle = int(math.Ceil(float64(lin) * 0.5))
				} else {
					middle = int(math.Ceil(float64(lin)*0.5)) - 1
				}
				chunk = append(chunk, in[middle])
				in2 = append(in2, middle)
			default:
				if x < 0 {
					return nil, ErrPositiveInt
				}
				chunk = append(chunk, in[x])
				// keep indexes removed from in
				in2 = append(in2, x)
			}
		}
		out = append(out, chunk...)

		if l == lin {
			return
		}

		// mark to remove in's elements collected in chunk
		for _, x := range in2 {
			// mark hole
			in[x] = -999
		}
		// rebuild  in but without elements collected in chunk
		in2 = []int{}
		for _, x := range in {
			if x != -999 {
				in2 = append(in2, x)
			}
		}
		// the new in that have lost some elements
		in = append([]int{}, in2...)
	}
	return out, nil
}
