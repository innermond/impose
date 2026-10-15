package impose

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
		// every sheet is drawn, cropmarks included, when the cycle returns
		defer close(counter)
		// cycle every page and draw it, the last sheet gets its cropmarks from the cycle itself
		bb.CycleAdjusted(pxp, counter, adjuster, showcropmarkPages)
	}()
	return counter
}
