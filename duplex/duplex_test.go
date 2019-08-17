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
		/*{4, 1, 2, false, false,
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{1, 2, 5, 6, 3, 4, 7, 8},
		},
		{4, 1, 2, false, true,
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{1, 2, 5, 6, 7, 8, 3, 4},
		},*/
		{4, 2, 2, false, true,
			span(1, 32),
			[]int{1, 2, 5, 6, 7, 8, 3, 4},
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
	out := []int{}
	for i := a; i <= z; i++ {
		out = append(out, i)
	}
	return out
}
