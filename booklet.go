package impose

import (
	"fmt"
	"log"

	"github.com/cheggaaa/pb/v3"
	"github.com/innermond/impose/duplex"
	"github.com/innermond/impose/reflow"
	"github.com/unidoc/unipdf/v3/creator"
)

func (bb *Boxes) Booklet(
	pxp []int,
	creep float64,
) {
	// proxy variables
	var (
		err        error
		np         = len(pxp)
		maxOnSheet = bb.Col * bb.Row
		left, top  = bb.Big.Left, bb.Big.Top
		xpos, ypos = left, top
		w, h       = bb.Small.Width, bb.Small.Height
	)

	if np%4 != 0 {
		log.Fatalf("%d is not divisible with 4", np)
	}

	pxp, err = reflow.On(pxp, []int{-1, 0, 1, -1})
	if err != nil {
		log.Fatal(err)
	}
	flip := true
	pxp, err = duplex.Reflow(pxp, 2, bb.Col, bb.Row, false, flip)
	if err != nil {
		log.Fatal(err)
	}

	// start imposition
	var (
		i         int
		nextSheet bool
	)
	bar := pb.StartNew(np)
	bb.NewSheet()
grid:
	for {
		for y := 0; y < bb.Row; y++ {
			for x := 0; x < bb.Col; x++ {
				if i >= np {
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
					ypos = top
					bb.NewSheet()
					nextSheet = false
				}
				if pxp[i] > 0 {
					bb.DrawPage(pxp[i], xpos, ypos)
				}
				// count pages processed
				i++
				xpos += float64(w)
				bar.Increment()
			}
			ypos += float64(h)
			xpos = left
		}
	}
	// put cropmarks for the last sheet
	bb.DrawCropmark()

	bar.Finish()
	// ring terminal bell once
	fmt.Print("\a\n")
}

func (bb *Boxes) DrawPage(num int, xpos, ypos float64) error {
	var (
		err   error
		w, h  = bb.Small.Width, bb.Small.Height
		angle = bb.Small.Angle

		bk                         *creator.Block
		i                          int
		dt, step                   float64
		nextSheet                  bool
		creepCount, nextSheetCount int
	)

	bk, err = bb.Reader.BlockFromPage(num)
	if err != nil {
		return err
	}

	// lay down imported page
	xposx, yposy := xpos, ypos
	if angle != 0.0 {
		bk.SetAngle(angle)
		if angle == -90.0 || angle == 270 {
			xposx += w
		}
		if angle == 90.0 || angle == -270 {
			yposy += h
		}
		if angle == -180 || angle == 180 {
			xposx += w
			yposy += h
		}
	}
	// time to check if block is front or verso
	if i > 2 && (i-1)%2 == 0 {
		if nextSheet {
			nextSheetCount++
			if nextSheetCount%2 == 0 {
				dt -= float64(creepCount) * step
			} else {
				dt += step
			}
			// reset counter
			creepCount = 0
		} else {
			creepCount++
			dt += step
		}
	}
	direction := 0.0
	if i%2 == 0 {
		direction = 1.0
	} else {
		direction = -1.0
	}

	switch angle {
	case 0.0:
		xposx += direction * dt
		bk.Clip(-1*direction*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case -90, 270:
		yposy += direction * dt
		bk.Clip(-direction*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case 90, -270:
		yposy += direction * dt
		bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case 180, -180:
		xposx += direction * dt
		bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	}
	// layout page
	bk.SetPos(xposx, yposy)
	_ = bb.Creator.Draw(bk)

	return err
}
