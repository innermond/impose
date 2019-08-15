package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/innermond/impose"
	"github.com/innermond/pange"
	"github.com/unidoc/unipdf/v3/creator"
)

const nothere = "duplex_value"

func main() {
	err := param()
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()

	// Read the input pdf file.
	f, err := os.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// read first pdf page
	log.Printf("read pdf of %q", fn)
	pdf, err := impose.NewReader(f, bleedx, bleedy)
	if err != nil {
		log.Fatal(err)
	}
	// establish pages number
	np, err := pdf.GetNumPages()
	if err != nil {
		log.Fatal(err)
	}
	// from 1 to last
	if pages == "" {
		pages = fmt.Sprintf("1-%d", np)
	}
	// get pages for imposition
	sel := pange.Selection(pages)
	ppp, err := sel.Split()
	if err != nil {
		log.Fatal(err)
	}
	// all pages as a slice of ints
	pags, err := sel.Full(ppp...)
	if err != nil {
		log.Fatal(err)
	}
	np = len(pags)

	// grid
	var (
		col int = 1
		row int = 1
	)

	// parse grid; has form like 2x1, at minimum 3 chars
	if len(grid) > 2 && strings.Contains(grid, "x") {
		colrow := strings.Split(grid, "x")
		if len(colrow) != 2 {
			log.Fatal(errors.New("grid length invalid"))
		}

		col, err = strconv.Atoi(colrow[0])
		if err != nil {
			log.Fatal(err)
		}
		row, err = strconv.Atoi(colrow[1])
		if err != nil {
			log.Fatal(err)
		}
	}

	// bookletMode need  certain grid and page order
	if bookletMode {
		if grid == "" {
			grid = "2x1"
			col, row = 2, 1
		}
	}

	log.Println("parsed parameters")

	// assume all pages has the same dimensions as first one
	page, err := pdf.GetPage(1)
	if err != nil {
		log.Fatal(err)
	}
	bbox, err := page.GetMediaBox()
	if err != nil {
		log.Fatal(err)
	}
	w := bbox.Urx - bbox.Llx
	h := bbox.Ury - bbox.Lly

	// cropmarks adds extra to dimensions
	extw := offx + markw
	exth := offy + markh

	beSwitched := angle == 90.0 || angle == -90 || angle == 270 || angle == -270
	if autopage {
		width = left + float64(col)*w + right + 2*extw + 2*autopadding
		height = top + float64(row)*h + bottom + 2*exth + 2*autopadding
		if beSwitched {
			width = left + float64(col)*h + right + 2*extw + 2*autopadding
			height = top + float64(row)*w + bottom + 2*exth + 2*autopadding
		}
	}

	// create a sheet page
	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})

	bigbox := &impose.BigBox{&impose.Box{width, height, top, right, bottom, left}}
	smallbox := &impose.SmallBox{&impose.Box{Width: w, Height: h}, angle}
	bb := &impose.Boxes{bigbox, smallbox, col, row, np, c, pdf, nil, outline}

	angled := false
	if beSwitched {
		bb.Small.Switch()
		angled = true
	}
	// centering by changing margins
	if centerx {
		bb.AdjustMarginCenteringAlongWidth()
	}
	if centery {
		bb.AdjustMarginCenteringAlongHeight()
	}
	// guess the grid
	if grid == "" {
		col, row := bb.GuessGrid()
		log.Fatalf("sugested grid %dx%d", col, row)
	}

	// start to lay down pages
	top, right, bottom, left = bb.Big.Top, bb.Big.Right, bb.Big.Bottom, bb.Big.Left
	col, row = bb.Col, bb.Row
	h, w = bb.Small.Height, bb.Small.Width

	// check if media is enough
	if !bb.EnoughWidth() {
		log.Fatalf("%d columns do not fit", bb.Col)
	}
	if !bb.EnoughHeight() {
		log.Fatalf("%d rows do not fit", bb.Row)
	}

	log.Println("prepared boxes")

	if showcropmark {
		bb.CreateCropmark(
			markw, markh,
			extw, exth,
			bleedx, bleedy,
			bookletMode,
			angled,
		)
	}

	// we have duplex but not explicit
	if len(duplex) == 0 {
		// reverse flow seen as 1 to col
		if len(flow) == 0 {
			for i := col; i > 0; i-- {
				duplex += strconv.Itoa(i) + ","
			}
			duplex = duplex[0 : len(duplex)-1]
		} else {
			for i := len(flow); i > 0; i += 2 {
				duplex += flow[i : i-1]
			}
		}
	} else if duplex == nothere {
		duplex = ""
	}

	if repeat {
		bb.Repeat(pags)
	} else if bookletMode {
		bb.Booklet(
			pags,
			creep,
		)
	} else {
		/*bb.Impose(flow, duplex,

			pags,
		)*/
	}

	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)

	log.Printf("time taken %v\n", elapsed)
	log.Printf("file %s written\n", fout)
}
