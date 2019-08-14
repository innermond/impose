package main

import (
	"fmt"
	"log"

	"github.com/cheggaaa/pb/v3"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

func (bb *Boxes) Repeat(
	pxp []int,
	cros2b *creator.Block,
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
		sheet      *model.PdfPage
		bk         *creator.Block
		i, j       int
		nextSheet  bool
		xpos, ypos = left, top
		num        int
	)
	sheet = model.NewPdfPage()
	sheet.MediaBox = &model.PdfRectangle{0, 0, bb.Big.Width, bb.Big.Height}
	c.AddPage(sheet)
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
				// num resulted larger than number of pages
				// place an empty space with the right wide
				if num > np {
					xpos += float64(w)
					continue
				}
				// get the page number from pages slice
				num = pxp[i]
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
				bk.Clip(0, 0, bk.Width(), bk.Height(), outline)
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
