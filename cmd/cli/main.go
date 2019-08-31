package main

import (
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
	// all pages as a slice of ints
	pags, err := sel.Full()
	if err != nil {
		log.Fatal(err)
	}
	np = len(pags)

	// grid
	var (
		col int = 1
		row int = 1
	)

	col, row, err = parsex(grid)
	if err != nil {
		log.Fatal("grid: ", err)
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
	clonex, cloney := 1, 1
	clonex, cloney, err = parsex(clone)
	if err != nil {
		log.Fatal("clone: ", err)
	}
	if autopage {
		width = left + float64(clonex)*float64(col)*w + right + 2*extw + 2*autopadding
		height = top + float64(cloney)*float64(row)*h + bottom + 2*exth + 2*autopadding
		if beSwitched {
			width = left + float64(clonex)*float64(col)*h + right + 2*extw + 2*autopadding
			height = top + float64(cloney)*float64(row)*w + bottom + 2*exth + 2*autopadding
		}
	}

	// create a sheet page
	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})

	bigbox := &impose.BigBox{
		Box: &impose.Box{
			Width:  width,
			Height: height,
			Top:    top,
			Right:  right,
			Bottom: bottom,
			Left:   left,
		},
	}
	smallbox := &impose.SmallBox{Box: &impose.Box{Width: w, Height: h}, Angle: angle}
	bb := &impose.Boxes{
		Big:      bigbox,
		Small:    smallbox,
		Col:      col,
		Row:      row,
		CloneX:   clonex,
		CloneY:   cloney,
		Num:      np,
		Creator:  c,
		Reader:   pdf,
		Cropmark: nil,
		Outline:  outline,
		DeltaPos: 0.0,
	}

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

	if repeat {
		bb.Repeat(pags)
	} else if bookletMode {
		bb.Booklet(
			pags,
			creep,
			flip, reverse, turn,
		)
	} else {
		flowArr := []int{}
		if len(flow) < 1 {
			flowArr = []int{0}
		} else {
			flowStr := strings.Split(flow, ",")
			for _, val := range flowStr {
				inx, err := strconv.Atoi(val)
				if err != nil {
					log.Fatal(err)
				}
				flowArr = append(flowArr, inx)
			}
		}
		bb.Impose(
			pags,
			flowArr,
			weld,
			flip, reverse, turn,
		)
	}

	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)

	log.Printf("time taken %v\n", elapsed)
	log.Printf("file %s written\n", fout)
}
