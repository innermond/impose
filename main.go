package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/innermond/impose/booklet"
	"github.com/innermond/pange"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

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
	pdfReader, err := model.NewPdfReader(f)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("read pdf of %q", fn)
	// establish pages number
	np, err := pdfReader.GetNumPages()
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
	pagInts, err := sel.Full(ppp...)
	if err != nil {
		log.Fatal(err)
	}
	np = len(pagInts)

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
		if len(pagInts)%4 != 0 {
			log.Fatalf("number of pages %d is not divisible with 4", np)
		}
		if grid == "" {
			grid = "2x1"
			col, row = 2, 1
		}
		pagInts, err = booklet.Arrange(col, row, pagInts)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("parsed parameters")

	// assume all pages has the same dimensions as first one
	page, err := pdfReader.GetPage(1)
	if err != nil {
		log.Fatal(err)
	}
	adjustMediaBox(page, bleedx, bleedy)
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
			width = left + float64(row)*h + right + 2*extw + 2*autopadding
			height = top + float64(col)*w + bottom + 2*exth + 2*autopadding
		}
	}

	bigbox := &BigBox{&Box{width, height, top, right, bottom, left}}
	smallbox := &SmallBox{&Box{Width: w, Height: h}}
	bb := &Boxes{bigbox, smallbox, col, row, np}

	angled := false
	if beSwitched {
		bb.SwitchGrid()
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

	// create a sheet page
	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})

	var cros2b *creator.Block
	if showcropmark {
		cropbk := &CropMarkBlock{w, h, bleedx, bleedy, col, row, extw, exth, c}
		cros2b = cropbk.Create(bookletMode, angled)
	}

	if repeat {
		numsheet := col * row
		np *= numsheet
		repeated := []int{}
		for _, e := range pagInts {
			for i := 0; i < numsheet; i++ {
				repeated = append(repeated, e)
			}
		}
		pagInts = repeated
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

	bb.Impose(flow, duplex, np, angle,
		pagInts,
		pdfReader, c,
		cros2b,
		bookletMode, creep,
		outline,
		bleedx, bleedy,
	)

	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)

	log.Printf("time taken %v\n", elapsed)
	log.Printf("file %s written\n", fout)
}
