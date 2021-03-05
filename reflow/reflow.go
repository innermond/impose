package reflow

import (
	"errors"
)

var (
	ErrPositiveInt = errors.New("got negative value")
)

const (
	end  = -1
	half = -2
)

func On(in []int, as []int) (out []int, err error) {
	l := len(as)
	// as long we have something to work on
	for len(in) > 0 {
		chunk := []int{}
		moved := []int{}
		lin := len(in)
		if l > lin {
			in = append(in, make([]int, l-lin)...)
			lin = len(in)
		}
		for _, x := range as {
			switch x {
			case end:
				lin--
				chunk = append(chunk, in[lin])
				moved = append(moved, lin)
			case half:
				middle := lin / 2
				chunk = append(chunk, in[middle])
				moved = append(moved, middle)
			default:
				x--
				if x < 0 {
					return nil, ErrPositiveInt
				}
				chunk = append(chunk, in[x])
				// keep indexes removed from in
				moved = append(moved, x)
			}
		}
		for _, x := range moved {
			// mark hole
			in[x] = -999
		}
		// in losts its elements
		moved = []int{}
		for _, x := range in {
			if x != -999 {
				moved = append(moved, x)
			}
		}
		in = moved

		out = append(out, chunk...)

	}
	return out, nil
}
