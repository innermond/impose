package booklet

import (
	"reflect"
	"testing"
)

func TestBooklet(t *testing.T) {
	tt := []struct {
		col, row int
		pp       []int
		want     []int
	}{
		{2, 1,
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{8, 1, 2, 7, 6, 3, 4, 5},
		},
		{2, 2,
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{8, 1, 6, 3, 2, 7, 4, 5},
		},
		{4, 1,
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{8, 1, 6, 3, 2, 7, 4, 5},
		},
		{4, 2,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			[]int{16, 1, 14, 3, 12, 5, 10, 7, 2, 15, 4, 13, 6, 11, 8, 9},
		},
		{6, 1,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			[]int{12, 1, 10, 3, 8, 5, 2, 11, 4, 9, 6, 7},
		},
		{2, 3,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			[]int{12, 1, 10, 3, 8, 5, 2, 11, 4, 9, 6, 7},
		},
	}

	for i, tc := range tt {
		got, err := Arrange(tc.col, tc.row, tc.pp)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%d) got %+v want %+v", i+1, got, tc.want)
		}
	}
}
