package main

import (
	"fmt"
	"log"

	"github.com/cheggaaa/pb/v3"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

func (bb *Boxes) Booklet(flow string,
	pxp []int,
	cros2b *creator.Block,
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
		pdfReader  = bb.Reader
		c          = bb.Creator
	)
	// start imposition
	var (
		sheet                      *model.PdfPage
		bk                         *creator.Block
		i, j                       int
		dt, step                   float64
		nextSheet                  bool
		xpos, ypos                 = left, top
		num                        int
		creepCount, nextSheetCount int
	)
	step = creep / float64(np/4)

	sheet = model.NewPdfPage()
	sheet.MediaBox = &model.PdfRectangle{0, 0, c.Width(), c.Height()}
	c.AddPage(sheet)
	nextSheetCount = 1

	// parse flow
	ff, err := bb.ParseFlow(flow)
	if err != nil {
		log.Fatal(err)
	}
	// count sheets
	sheets := 1
	bar := pb.StartNew(np)
grid:
	for {
		for y := 0; y < row; y++ {
			for x := 0; x < len(ff); x++ {
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
					if cros2b != nil {
						c.Draw(cros2b)
					}
					// initialize position
					ypos = top
					//c.NewPage()
					sheet = model.NewPdfPage()
					sheet.MediaBox = &model.PdfRectangle{0, 0, c.Width(), c.Height()}
					c.AddPage(sheet)
				}
				num = ff[x] + j*col
				// num resulted larger than number of pages
				// place an empty space with the right wide
				if num > np {
					xpos += float64(w)
					continue
				}
				// get the page number from pages slice
				num = pxp[num-1]

				// count pages processed
				i++

				bk, err = pdfReader.BlockFromPage(num)
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
					bk.Clip(-1*direction*dt, 0, bk.Width(), bk.Height(), outline)
				case -90, 270:
					yposy += direction * dt
					bk.Clip(-direction*dt, 0, bk.Width(), bk.Height(), outline)
				case 90, -270:
					yposy += direction * dt
					bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), outline)
				case 180, -180:
					xposx += direction * dt
					bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), outline)
				}
				// layout page
				bk.SetPos(xposx, yposy)
				_ = c.Draw(bk)

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
	if cros2b != nil {
		c.Draw(cros2b)
	}

	bar.Finish()
	// ring terminal bell once
	fmt.Print("\a\n")
}
