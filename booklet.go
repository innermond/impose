package impose

import (
	"fmt"
	"log"
	"math"

	"github.com/cheggaaa/pb/v3"
	"github.com/innermond/impose/duplex"
	"github.com/innermond/impose/reflow"
)

func (bb *Boxes) Booklet(
	pxp []int,
	creep float64,
	flip, reverse bool,
	turn float64,
) {
	// proxy variables
	var (
		err error
		np  = len(pxp)
	)

	if np%4 != 0 {
		log.Fatalf("%d is not divisible with 4", np)
	}
	// booklet signature {last first second second-last}
	// example: 4 pages are imposed like this 4-1-2-3
	// 4-1 are a front page 2-3 are corespondent back page - the duplex
	// reflow order pages for booklet
	pxp, err = reflow.On(pxp, []int{-1, 0, 1, -1})
	if err != nil {
		log.Fatal(err)
	}
	// calculate creeping as ints with a multiplier because go do not have generics
	// and our duplex.Reflow accepts []int not []float64
	creepx := []int{}
	// for booklet two pages make a unit, they are "welded"
	weld := 2
	// calculate creep step
	dx := creep / float64(len(pxp))
	multiplier := 100.0
	// round to nearest
	dx = math.Round(dx*multiplier) / multiplier
	dxint := int(dx * multiplier)
	// step every welded elements - face + back so step is 2*weld
	for i := 0; i < len(pxp); i += 2 * weld {
		creepx = append(creepx, i*dxint)
		creepx = append(creepx, -i*dxint)
		creepx = append(creepx, i*dxint)
		creepx = append(creepx, -i*dxint)
	}
	// reverse+flip are args
	// when duplex is flip-ed (along printing direction - long edge in most cases)
	//reverse := false
	//flip := false
	// when duplex is turn-ed (crossing printing direction - short edge in most cases)
	// duplex pages must be rotated 180
	// calculate creep to coresponds with duplexed pxp
	creepx, err = duplex.Reflow(creepx, weld, bb.Col, bb.Row, reverse, flip)
	if err != nil {
		log.Fatal(err)
	}
	pxp, err = duplex.Reflow(pxp, weld, bb.Col, bb.Row, reverse, flip)
	if err != nil {
		log.Fatal(err)
	}

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

	adjuster := bb.Adjuster(turn, creepx, multiplier)
	// cycle every page and draw it
	bb.CycleAdjusted(pxp, counter, adjuster)
	// put cropmarks for the last sheet
	bb.DrawCropmark()
}
