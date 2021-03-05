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
	for len(in) > 0 {
		chunk := []int{}
		in2 := []int{}
		for _, x := range as {
			lin := len(in)
			if l > lin {
				in = append(in, make([]int, l-lin)...)
				lin = len(in)
			}
			//fmt.Println("x", x)
			switch x {
			case end:
				lin--
				chunk = append(chunk, in[lin])
				in2 = append([]int{}, in2[:lin]...)
			case half:
				middle := int(math.Ceil(float64(lin)*0.5)) - 1
				chunk = append(chunk, in[middle])
				in2 = append(in2[:middle], in2[middle+1:]...)
			default:
				x--
				//fmt.Println("default x", x, "in[x]", in[x])
				if x < 0 {
					return nil, ErrPositiveInt
				}
				chunk = append(chunk, in[x])
				// keep indexes removed from in
				in2 = append(in2, x)
			}
		}
		for _, x := range in2 {
			// mark hole
			in[x] = -999
		}
		// in losts its elements
		in2 = []int{}
		for _, x := range in {
			if x != -999 {
				in2 = append(in2, x)
			}
		}
		in = in2

		out = append(out, chunk...)

	}
	return out, nil
}
