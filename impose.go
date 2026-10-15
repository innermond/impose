package impose

import (
	"log"

	"github.com/innermond/impose/duplex"
	"github.com/innermond/impose/reflow"
)

func (bb *Boxes) Impose(
	pxp []int,
	flow []int,
	weld int,
	flip, reverse bool,
	turn float64,
	is_duplex bool,
	showcropmarkPages []int,
) chan int {
	var err error

	// TODO duplex command or flag
	if is_duplex {
		pxp, err = duplex.Reflow(pxp, weld, bb.Col, bb.Row, reverse, flip)
		if err != nil {
			log.Fatal(err)
		}
	}

	bb.Num = len(pxp)

	// flow is 1-based and can repeat pages, it is applied on every block of max(flow) pages
	pxp, err = reflow.Block(pxp, flow)
	if err != nil {
		log.Fatal(err)
	}

	bb.Num = len(pxp)

	adjuster := bb.Rotator(turn)
	counter := make(chan int)
	go func() {
		// every sheet is drawn, cropmarks included, when the cycle returns
		defer close(counter)
		// cycle every page and draw it, the last sheet gets its cropmarks from the cycle itself
		bb.CycleAdjusted(pxp, counter, adjuster, showcropmarkPages)
	}()

	return counter

}
