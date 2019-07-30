package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/innermond/pange"
	"github.com/unidoc/unipdf/v3/creator"
	pdf "github.com/unidoc/unipdf/v3/model"
)

var (
	fn                       string
	fout                     string
	width                    float64
	height                   float64
	unit                     string
	top, left, bottom, right float64
	center, centerx, centery bool
	lessPagesNum             int
	pages                    string
	postfix                  string
	samepage                 int
	grid                     string
	flow                     string
	angle                    float64
	bleed, bleedx, bleedy    float64
	offset, offx, offy       float64
	marksize, markw, markh   float64
	booklet                  bool
	creep                    float64
	outline                  bool
)

func param() error {
	var err error

	flag.StringVar(&fn, "f", "", "source pdf file")
	flag.StringVar(&fout, "o", "", "imposition pdf file")
	flag.Float64Var(&width, "width", 320.0, "imposition sheet width")
	flag.Float64Var(&height, "height", 450.0, "imposition sheet height")
	flag.StringVar(&unit, "unit", "mm", "unit of measurements")
	flag.Float64Var(&top, "top", 0.0, "top margin")
	flag.Float64Var(&left, "left", 0.0, "left margin")
	flag.Float64Var(&bottom, "bottom", 0.0, "bottom margin")
	flag.Float64Var(&right, "right", 0.0, "right margin")
	flag.BoolVar(&center, "center", false, "center along sheet axes")
	flag.BoolVar(&centerx, "centerx", false, "center along sheet width")
	flag.BoolVar(&centery, "centery", false, "center along sheet height")
	flag.IntVar(&lessPagesNum, "less", 0, "number of pages to be subject of imposition")
	flag.StringVar(&pages, "pages", "", "pages requested by imposition")
	flag.StringVar(&postfix, "postfix", "imposition", "final page termination")
	flag.IntVar(&samepage, "samepage", 0, "page chosen to repeat")
	flag.StringVar(&grid, "grid", "", "imposition layout columns x  rows. ex: 2x3")
	flag.StringVar(&flow, "flow", "", "it works along with grid flag. how pages are ordered on every row, they are flowing from 1 to col, but that can be changed, ex: 4,2,1,3")
	flag.Float64Var(&angle, "angle", 0.0, "angle to angle pages")
	flag.Float64Var(&offset, "offset", 2.0, "distance cut mark keeps from the last edge")
	flag.Float64Var(&offx, "offx", 2.0, " axe x distance cut mark keeps from the last edge")
	flag.Float64Var(&offy, "offy", 2.0, " axe y distance cut mark keeps from the last edge")
	flag.Float64Var(&bleed, "bleed", 2.0, "distance cut mark has been given in respect to the last edge")
	flag.Float64Var(&bleedx, "bleedx", 2.0, "axe x distance cut mark has been given in respect to the last edge")
	flag.Float64Var(&bleedy, "bleedy", 2.0, "axe y distance cut mark has been given in respect to the last edge")
	flag.Float64Var(&marksize, "marksize", 5.0, "cut mark size")
	flag.Float64Var(&markw, "markw", 5.0, "axe x cut mark size")
	flag.Float64Var(&markh, "markh", 5.0, "axe y cut mark size")
	flag.BoolVar(&booklet, "booklet", false, "booklet signature")
	flag.Float64Var(&creep, "creep", 0.0, "adjust imposition to deal with sheet's tickness")
	flag.BoolVar(&outline, "outline", false, "draw a containing rect around imported page")

	flag.Parse()

	if fn == "" {
		return errors.New("pdf file required")
	}

	// all to points
	left *= creator.PPMM
	right *= creator.PPMM
	top *= creator.PPMM
	bottom *= creator.PPMM
	width *= creator.PPMM
	height *= creator.PPMM
	offset *= creator.PPMM
	offx *= creator.PPMM
	offy *= creator.PPMM
	bleed *= creator.PPMM
	bleedx *= creator.PPMM
	bleedy *= creator.PPMM
	marksize *= creator.PPMM
	markw *= creator.PPMM
	markh *= creator.PPMM

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "centerx":
			centerx = true
		case "centery":
			centery = true
		case "center":
			centerx = center
			centery = center
		case "offset":
			offx = offset
			offy = offset
		case "marksize":
			markw = marksize
			markh = marksize
		case "bleed":
			bleedx = bleed
			bleedy = bleed
		case "booklet":
			booklet = true
			creep *= creator.PPMM
			if creep > bleedx {
				creep = bleedx
			}
		}
	})

	// last edge is further inside mediabox by bleed amount
	// adjunst offsets otherwise they will be aware only by media edge
	offx -= bleedx
	offy -= bleedy

	postfix = fmt.Sprintf(".%s", strings.TrimSpace(strings.Trim(postfix, ".")))

	if fout == "" {
		ext := path.Ext(fn)
		fout = fn[:len(fn)-len(ext)] + postfix + ext
	}

	if !booklet {
		creep = 0.0
	}

	return err
}

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

	pdfReader, err := pdf.NewPdfReader(f)
	if err != nil {
		log.Fatal(err)
	}

	np, err := pdfReader.GetNumPages()
	if err != nil {
		log.Fatal(err)
	}

	if lessPagesNum > 0 && lessPagesNum < np {
		np = lessPagesNum
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
	pxp, err := sel.Full(ppp...)
	if err != nil {
		log.Fatal(err)
	}

	var (
		col int = 1
		row int = 1
	)

	if booklet {
		grid = "2x1"
	}
	// parse grid; has form like 2x1, at minimum 3 chars
	if len(grid) > 2 {
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

	// assume all pages has the same dimensions as first one
	page, err := pdfReader.GetPage(1)
	if err != nil {
		log.Fatal(err)
	}
	bbox, err := page.GetMediaBox()
	if err != nil {
		log.Fatal(err)
	}
	w := bbox.Urx - bbox.Llx
	h := bbox.Ury - bbox.Lly
	fmt.Println(w, h, bbox)
	bigbox := &BigBox{&Box{width, height, top, right, bottom, left}}
	smallbox := &SmallBox{&Box{Width: w, Height: h}}
	bb := &Boxes{bigbox, smallbox, col, row}

	if angle == 90.0 || angle == -90 || angle == 270 || angle == -270 {
		bb.SwitchGrid()
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

	// rearange ppp suitable for booklet signature
	if booklet {
		if len(pxp)%4 != 0 {
			log.Fatalf("number of pages %d is not divisible with 4", len(pxp))
		}
		book := []int{}
		for len(pxp) > 0 {
			z, a, b, y := pxp[len(pxp)-1], pxp[0], pxp[1], pxp[len(pxp)-2]
			book = append(book, z, a, b, y)
			pxp = pxp[2 : len(pxp)-2]
		}
		pxp = book
	}

	// check if media is enough
	if !bb.EnoughWidth() {
		log.Fatalf("%d columns do not fit", bb.Col)
	}
	if !bb.EnoughHeight() {
		log.Fatalf("%d rows do not fit", bb.Row)
	}

	// clamp number of pages
	np = len(pxp)
	if lessPagesNum > 0 && lessPagesNum < np {
		np = lessPagesNum
	}

	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})
	// cropmarks adds extra to dimensions
	extw := offx + markw
	exth := offy + markh
	cropbk := &CropMarkBlock{w, h, bleedx, bleedy, col, row, extw, exth, c}
	cros2b := cropbk.Create()

	bb.Impose(flow, np, angle, pxp, pdfReader, c, cros2b, booklet, creep, outline)

	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)

	log.Printf("time taken %v\n", elapsed)
	log.Printf("file %s written\n", fout)
}

func getFlowAsInts(ss []string, max int) (list []int, err error) {
	keys := make(map[int]bool)
	var (
		i int
		e string
	)
	for _, e = range ss {
		i, err = strconv.Atoi(e)
		if err != nil {
			return
		}
		if i > max {
			err = fmt.Errorf("max flow is %d element %d unacceptable", max, i)
			return
		}
		if _, ok := keys[i]; !ok {
			keys[i] = true
			list = append(list, i)
		}
	}
	return
}

// round floor to floor
// go has a quirks regarding floor numbers
// identical numbers may have their remote decimals different
// so comparing them for equality is compromised
// but we restore order by keeping only a specified number of decimals
func floor63(v float64, p ...int) float64 {
	a := 2
	if len(p) > 0 {
		a = p[0]
	}
	n := math.Pow10(a)
	return math.Floor(v*n) / n
}
