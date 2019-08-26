package impose

import (
	"fmt"
	"log"
	"math"

	"github.com/cheggaaa/pb/v3"
	"github.com/innermond/impose/duplex"
	"github.com/innermond/impose/reflow"
	"github.com/unidoc/unipdf/v3/creator"
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

func (bb *Boxes) Rotator(turn float64) func(int) {
	var isBack, isFace, stilBack, stilFace bool
	return func(i int) {
		if turn != 0.0 && i >= bb.Col*bb.Row {
			// is on a duplex page ? all 2th page is duplex
			zero := int(math.Ceil(float64(i+1)/float64(bb.Col*bb.Row))) % 2

			isBack = zero == 0
			isFace = !isBack

			if !stilBack && isBack {
				stilBack = true
				stilFace = false
				bb.Small.Angle -= turn
			} else if !stilFace && isFace {
				stilBack = false
				stilFace = true
				bb.Small.Angle += turn
			}
		}
	}
}

func (bb *Boxes) Adjuster(turn float64, creepx []int, multiplier float64) func(int) {
	var isBack, isFace, stilBack, stilFace bool
	return func(i int) {
		if turn != 0.0 && i >= bb.Col*bb.Row {
			// is on a duplex page ? all 2th page is duplex
			zero := int(math.Ceil(float64(i+1)/float64(bb.Col*bb.Row))) % 2

			isBack = zero == 0
			isFace = !isBack

			if !stilBack && isBack {
				stilBack = true
				stilFace = false
				bb.Small.Angle -= turn
			} else if !stilFace && isFace {
				stilBack = false
				stilFace = true
				bb.Small.Angle += turn
			}
		}
		bb.DeltaPos = float64(creepx[i]) / multiplier
	}
}

// proxy func
func (bb *Boxes) Cycle(pxp []int, c chan int) {
	bb.CycleAdjusted(pxp, c, nil)
}

func (bb *Boxes) CycleAdjusted(pxp []int, c chan int, adjuster func(i int)) {
	var (
		err        error
		maxOnSheet = bb.Col * bb.Row
		xpos, ypos = bb.Big.Left, bb.Big.Top
		w, h       = bb.Small.Width, bb.Small.Height
		i          int
		nextSheet  bool
	)
	// start imposition
	bb.NewSheet()
grid:
	for {
		for y := 0; y < bb.Row; y++ {
			for x := 0; x < bb.Col; x++ {
				if i >= bb.Num {
					break grid
				}
				// check the need for a new page
				if i >= maxOnSheet {
					nextSheet = (maxOnSheet+i)%maxOnSheet == 0
				}
				if nextSheet {
					// put cropmarks on sheet
					bb.DrawCropmark()
					// initialize position
					ypos = bb.Big.Top
					bb.NewSheet()
					nextSheet = false
				}
				if pxp[i] > 0 {
					if adjuster != nil {
						adjuster(i)
					}
					err = bb.DrawPage(pxp[i], xpos, ypos)
					if err != nil {
						log.Fatal(err)
					}
				}
				// count pages processed
				i++
				// signal page drawing
				c <- i
				xpos += float64(w)
			}
			ypos += float64(h)
			xpos = bb.Big.Left
		}
	}
	close(c)
}

func (bb *Boxes) DrawPage(num int, xpos, ypos float64) error {
	var (
		err   error
		w, h  = bb.Small.Width, bb.Small.Height
		angle = bb.Small.Angle
		dt    = bb.DeltaPos

		bk *creator.Block
	)

	bk, err = bb.Reader.BlockFromPage(num)
	if err != nil {
		return err
	}

	// lay down imported page
	xposx, yposy := xpos, ypos
	bk.SetAngle(angle)
	// bk is top left corner oriented by framework choice
	// Clip is bottom right oriented by pdf specification
	// angle is counter clock wise, so -90 is clock wise
	// do the math!!!
	switch angle {
	case 0.0:
		xposx += dt
		bk.Clip(-1*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case -90, 270:
		xposx += w
		xposx += dt
		bk.Clip(0, -dt, bk.Width(), bk.Height(), bb.Outline)
	case 90, -270:
		yposy += h
		xposx += dt
		bk.Clip(0, dt, bk.Width(), bk.Height(), bb.Outline)
	case 180, -180:
		xposx += w
		yposy += h
		xposx += dt
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
	}
	// layout page
	bk.SetPos(xposx, yposy)
	_ = bb.Creator.Draw(bk)

	return err
}
