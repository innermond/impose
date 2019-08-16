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
		{2, 2, 2, false, false,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			[]int{1, 2, 5, 6, 7, 8, 3, 4},
		},
	}

	for i, tc := range tt {
		got, err := Reflow(tc.in, tc.weld, tc.col, tc.row, tc.reversed, tc.flip)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%d) got %+v want %+v", i+1, got, tc.want)
		}
	}
}
