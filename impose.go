package impose

import (
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
	progress func(int),
) error {
	var err error

	// TODO duplex command or flag
	if is_duplex {
		pxp, err = duplex.Reflow(pxp, weld, bb.Col, bb.Row, reverse, flip)
		if err != nil {
			return err
		}
	}

	bb.Num = len(pxp)

	// flow is 1-based and can repeat pages, it is applied on every block of max(flow) pages
	pxp, err = reflow.Block(pxp, flow)
	if err != nil {
		return err
	}

	bb.Num = len(pxp)

	adjuster := bb.Rotator(turn)
	// cycle every page and draw it, the last sheet gets its cropmarks from the cycle itself
	_, err = bb.CycleAdjusted(pxp, progress, adjuster, showcropmarkPages)
	return err
}
