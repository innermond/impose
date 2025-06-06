package impose

import (
	"slices"
)

func (bb *Boxes) Repeat(
	pxp []int,
	turn float64,
	showcropmarkPages []int,
) chan int {
	// proxy variables
	bb.Num = len(pxp)

	counter := make(chan int)
	adjuster := bb.Rotator(turn)
	go func() {
		// cycle every page and draw it
		n := bb.CycleAdjusted(pxp, counter, adjuster, showcropmarkPages)
		// put cropmarks for the last sheet
		if showcropmarkPages == nil || slices.Contains(showcropmarkPages, n) {
			bb.DrawCropmark()
		}
	}()
	return counter
}
