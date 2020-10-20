package impose

func (bb *Boxes) Repeat(
	pxp []int,
  turn float64,
) chan int {
	// proxy variables
	bb.Num = len(pxp)

	counter := make(chan int)
	adjuster := bb.Rotator(turn)
	go func() {
		// cycle every page and draw it
		bb.CycleAdjusted(pxp, counter, adjuster)
		// put cropmarks for the last sheet
		//bb.DrawCropmark()
	}()
	return counter
}
