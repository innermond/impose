package impose

func (bb *Boxes) Repeat(
	pxp []int,
	turn float64,
	showcropmarkPages []int,
	progress func(int),
) error {
	// proxy variables
	bb.Num = len(pxp)

	adjuster := bb.Rotator(turn)
	// cycle every page and draw it, the last sheet gets its cropmarks from the cycle itself
	_, err := bb.CycleAdjusted(pxp, progress, adjuster, showcropmarkPages)
	return err
}
