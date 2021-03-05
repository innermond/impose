package reflow

import (
	"reflect"
	"testing"
)

func TestOn(t *testing.T) {
	tt := []struct {
		in   []int
		as   []int
		want []int
	}{
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{-1, 1, 2, 3}, // 1 indexed  user friendly
			[]int{8, 1, 2, 3, 7, 4, 5, 6},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{1, -1, 2, 3},
			[]int{1, 8, 2, 3, 4, 7, 5, 6},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9}, // in asymetric to as complete with zeros
			[]int{-1, 1, 2, 3},
			[]int{9, 1, 2, 3, 8, 4, 5, 6, 0, 7, 0, 0},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{-1},
			[]int{8, 7, 6, 5, 4, 3, 2, 1},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{2, 1},
			[]int{2, 1, 4, 3, 6, 5, 8, 7},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
			[]int{1},
			[]int{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			[]int{1, 2, 3, 4, 5, 6},
			[]int{1, -2},
			[]int{1, 4, 2, 5, 3, 6},
		},
		{
			[]int{1, 2, 3, 4, 5, 6, 7},
			[]int{1, -2},
			[]int{1, 4, 2, 5, 3, 6, 7, 0},
		},
	}

	for i, tc := range tt {
		got, err := On(tc.in, tc.as)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%d) got %+v want %+v", i+1, got, tc.want)
		}
	}
}
