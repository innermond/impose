package impose

import (
	"fmt"

	"github.com/cheggaaa/pb/v3"
)

func (bb *Boxes) Repeat(
	pxp []int,
) {
	// proxy variables
	bb.Num = len(pxp)

	// decouple progress bar by drawing mechanics
	counter := make(chan int)
	go func() {
		bar := pb.StartNew(bb.Num)
		for {
			_, more := <-counter
			if more {
				bar.Increment()
			} else {
				break
			}
		}
		bar.Finish()
		// ring terminal bell once
		fmt.Print("\a\n")
	}()

	// cycle every page and draw it
	bb.Cycle(pxp, counter)
	// put cropmarks for the last sheet
	bb.DrawCropmark()
}
