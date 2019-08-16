package impose

import (
	"fmt"
	"log"

	"github.com/cheggaaa/pb/v3"
	"github.com/innermond/impose/booklet"
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
		col, row   = bb.Col, bb.Row
		maxOnSheet = col * row
		left, top  = bb.Big.Left, bb.Big.Top
		w, h       = bb.Small.Width, bb.Small.Height
		angle      = bb.Small.Angle
	)
	if np%4 != 0 {
		log.Fatalf("%d is not divisible with 4", np)
	}
	// start imposition
	var (
		bk                         *creator.Block
		i, j                       int
		dt, step                   float64
		nextSheet                  bool
		xpos, ypos                 = left, top
		num                        int
		creepCount, nextSheetCount int
	)
	step = creep / float64(np/4)

	bb.NewSheet()
	nextSheetCount = 1

	pxp, err = booklet.Arrange(col, row, pxp)
	if err != nil {
		log.Fatal(err)
	}
	// count sheets
	sheets := 1
	bar := pb.StartNew(np)
grid:
	for {
		for y := 0; y < row; y++ {
			for x := 0; x < col; x++ {
				if i >= np {
					break grid
				}
				// check the need for a new page
				if i >= maxOnSheet {
					nextSheet = (maxOnSheet+i)%maxOnSheet == 0
				}
				if nextSheet {
					// count sheets
					sheets++
					// put cropmarks on sheet
					bb.DrawCropmark()
					// initialize position
					ypos = top
					bb.NewSheet()
				}
				num = pxp[i]
				// count pages processed
				i++
				bk, err = bb.Reader.BlockFromPage(num)
				if err != nil {
					log.Fatal(err)
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
					direction = -1.0
				} else {
					direction = 1.0
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

				xpos += float64(w)
				if nextSheet {
					nextSheet = false
				}
				bar.Increment()
			}
			ypos += float64(h)
			xpos = left
			j++
		}
	}
	// put cropmarks for the last sheet
	bb.DrawCropmark()

	bar.Finish()
	// ring terminal bell once
	fmt.Print("\a\n")
}
