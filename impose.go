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
) chan int {
	// proxy variables
	var (
		err error
		np  = len(pxp)
	)

// TODO duplex command or flag
  if is_duplex {
    pxp, err = duplex.Reflow(pxp, weld, bb.Col, bb.Row, reverse, flip)
    if err != nil {
      log.Fatal(err)
    }
  }

	bb.Num = len(pxp)

	if bb.Num%len(flow) != 0 {
		log.Fatalf("%d is not divisible with %d", np, len(flow))
	}
	// booklet signature {last first second second-last}
	// example: 4 pages are imposed like this 4-1-2-3
	// 4-1 are a front page 2-3 are corespondent back page - the duplex
	// reflow order pages for booklet
	pxp, err = reflow.On(pxp, flow)
	if err != nil {
		log.Fatal(err)
	}

	bb.Num = len(pxp)

	adjuster := bb.Rotator(turn)
	counter := make(chan int)
	go func() {
		// cycle every page and draw it
		bb.CycleAdjusted(pxp, counter, adjuster)
		// put cropmarks for the last sheet
		bb.DrawCropmark()
	}()

	return counter

}
