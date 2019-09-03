package impose

func (bb *Boxes) Repeat(
	pxp []int,
) chan int {
	// proxy variables
	bb.Num = len(pxp)

	counter := make(chan int)
	go func() {
		// cycle every page and draw it
		bb.Cycle(pxp, counter)
		// put cropmarks for the last sheet
		bb.DrawCropmark()
	}()
	return counter
}
