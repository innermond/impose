package duplex

import (
	"reflect"
	"testing"
)

func TestReflow(t *testing.T) {
	tt := []struct {
		col, row, weld int
		reversed, flip bool
		in             []int
		want           []int
	}{
		// nothing changes here
		{1, 1, 1, false, false,
			span(1, 8),
			span(1, 8),
		},
		// columns
		{2, 1, 1, false, false,
			span(1, 8),
			[]int{1, 3, 2, 4, 5, 7, 6, 8},
		},
		// columns rows
		{2, 2, 1, false, false,
			span(1, 8),
			[]int{1, 3, 5, 7, 2, 4, 6, 8},
		},
		// one row of two elements
		{2, 1, 2, false, false,
			span(1, 8),
			span(1, 8),
		},
	}

	for i, tc := range tt {
		got, err := Reflow(tc.in, tc.weld, tc.col, tc.row, tc.reversed, tc.flip)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.in) {
			t.Errorf("%d) len got different than in %+v  %+v", i+1, len(got), len(tc.in))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Logf(" len got %v\n", len(got))
			t.Errorf("%d) got %+v want %+v", i+1, got, tc.want)
		}
	}
}

func span(a, z int) []int {

	less := func(i, z int) bool {
		return i <= z
	}
	more := func(i, z int) bool {
		return i >= z
	}
	compare := less

	up := func(i *int) {
		(*i)++
	}
	down := func(i *int) {
		(*i)--
	}
	mut := up

	if a > z {
		compare = more
		mut = down
	}

	out := []int{}
	for i := a; compare(i, z); mut(&i) {
		out = append(out, i)
	}
	return out
}
