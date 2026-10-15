package reflow

import (
	"errors"
	"math"
)

var (
	ErrPositiveInt = errors.New("only positive int")
	ErrFlow        = errors.New("flow needs at least one element and every element must be 1 or more")
	ErrNoProgress  = errors.New("flow points outside of the pages, no page can be taken")
)

func On(in []int, as []int) (out []int, err error) {
	const (
		end   = -1
		half  = -2
		odd   = -3
		even  = -4
		empty = 0
	)

	var (
		lenas = len(as)
		as0   = make([]int, lenas)
		el    int
	)
	copy(as0, as)

	// only positive ints
	for _, i := range in {
		if i < 0 {
			return nil, ErrPositiveInt
		}
	}
flow:
	for {
		// as was modified in order to cope with decrementing
		// we refresh it
		copy(as, as0)
		consumed := len(in)
		for i := 0; i < lenas; i++ {
			// consumed, no more work here
			if len(in) == 0 {
				break flow
			}

			// asked index
			inx := as[i]

			// here inx can point outside of in
			// so add empty value
			if len(in) <= inx {
				out = append(out, empty)
				continue
			}

			// special case end
			if inx == end {
				inx = len(in) - 1
				el = in[inx]
				out = append(out, el)
				in = append([]int{}, in[:inx]...)
				continue
			}
			// special case half
			if inx == half {
				inx = int(math.Ceil(float64(len(in))*0.5)) - 1
				el = in[inx]
				out = append(out, el)
				in = append(in[:inx], in[inx+1:]...)
				continue
			}

			// regular cases
			el = in[inx]
			// grow
			out = append(out, el)
			// shrink
			in = append(in[:inx], in[inx+1:]...)

			// what is over inx next iteration should be decremented
			for x := 0; x < lenas; x++ {
				if as[x] >= inx && as[x] != 0 {
					as[x]--
				}
			}
		}
		// a whole pass took nothing from in, it would loop forever
		if len(in) == consumed {
			return nil, ErrNoProgress
		}
	}
	return
}

// Block applies a 1-based flow, that may repeat pages, to every block of max(flow) pages of in.
// Pages missing from the last block are replaced by 0, an empty element.
func Block(in, flow []int) (out []int, err error) {
	if len(flow) == 0 {
		return nil, ErrFlow
	}
	size := 0
	for _, v := range flow {
		if v < 1 {
			return nil, ErrFlow
		}
		size = max(size, v)
	}
	for start := 0; start < len(in); start += size {
		for _, v := range flow {
			inx := start + v - 1
			if inx < len(in) {
				out = append(out, in[inx])
			} else {
				out = append(out, 0)
			}
		}
	}
	return
}
