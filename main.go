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
		}
	})

	offx -= bleedx
	offy -= bleedy

	postfix = fmt.Sprintf(".%s", strings.TrimSpace(strings.Trim(postfix, ".")))

	if fout == "" {
		ext := path.Ext(fn)
		fout = fn[:len(fn)-len(ext)] + postfix + ext
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

	// set default page media
	c := creator.New()
	media := creator.PageSize{width, height}
	c.SetPageSize(media)
	// Margins of page
	c.SetPageMargins(left, right, top, bottom)

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

	if angle == 90.0 || angle == -90 || angle == 270 || angle == -270 {
		w, h = h, w
	}

	c.NewPage()

	var (
		xpos, ypos, endx, endy, peakx, peaky float64
		col                                  int
		row                                  int = 1
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

	// centering by changing margins
	if centerx {
		wpages := w
		available := media[0] - (left + right)
		i := 0
		for wpages < available {
			wpages += w
			// sensible to grid
			i++
			if col > 0 && i == col {
				break
			}
		}
		wpages -= w
		left = (width - wpages) * 0.5
		right = left
	}
	if centery {
		hpages := h
		available := media[1] - (top + bottom)
		i := 0
		for hpages < available {
			hpages += h
			// sensible to grid
			i++
			if i == row {
				break
			}
		}
		hpages -= h
		top = (height - hpages) * 0.5
		bottom = top
	}

	// start to lay down pages
	xpos = left
	ypos = top

	// get pages for imposition
	ppp, err := pange.Selection(pages).Split()
	if err != nil {
		log.Fatal(err)
	}

	// collect pages as a slice of ints
	var pxp []int
	for _, pp := range ppp {
		for p := pp.A; p <= pp.Z; p++ {
			pxp = append(pxp, p)
		}
	}

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

	// guess the grid
	if grid == "" {
		var stopCountingCol bool
	guessing_grid:
		for _, pp := range ppp {
			for i := pp.A; i <= pp.Z; i++ {
				if i > np {
					break guessing_grid
				}
				endx = floor63(xpos + float64(w))
				peakx = floor63(media[0] - right)
				if endx > peakx {
					stopCountingCol = true
					xpos = left
					ypos += float64(h)
					endy = floor63(ypos + float64(h))
					peaky = floor63(media[1] - bottom)
					if endy > peaky {
						break guessing_grid
					}
					row++
				}
				xpos += float64(w)
				if !stopCountingCol {
					col++
				}
			}
		}
		log.Fatalf("sugested grid %dx%d\n", col, row)
	}

	// check if media is enough
	if left+right+float64(col)*w > media[0] {
		log.Fatalf("%d columns do not fit", col)
	}
	if top+bottom+float64(row)*h > media[1] {
		log.Fatalf("%d rows do not fit", row)
	}

	// parse flow
	var ff []int
	if flow != "" {
		ff, err = getFlowAsInts(strings.Split(flow, ","), col)
		if err != nil {
			log.Fatal(err)
		}
		if len(ff) != col {
			log.Fatal(fmt.Errorf("number of flow elements should be equal with %d", col))
		}
	} else {
		for i := 1; i <= col; i++ {
			ff = append(ff, i)
		}
	}

	var nextPage bool
	var maxOnPage = col * row

	// clamp number of pages
	np = len(pxp)
	if lessPagesNum > 0 && lessPagesNum < np {
		np = lessPagesNum
	}

	cros2bw := left + float64(col)*w + right
	cros2bh := top + float64(row)*h + bottom
	// create cropmarks block
	crosb := creator.NewBlock(cros2bw, cros2bh)
	crosb.SetPos(0.0, 0.0)

	// thw width used for cropmark
	lw := 0.4 * creator.PPMM // points

	// create top line of cropmarks and left line
	for y := 0; y < row; y++ {
		for x := 0; x < col; x++ {
			if y == 0 {
				l := c.NewLine(left+float64(x)*w+bleedx-0.5*lw, top-offy, left+float64(x)*w+bleedx-0.5*lw, top-offy-markh)
				l.SetLineWidth(lw)
				crosb.Draw(l)
				l = c.NewLine(left+float64(x+1)*w-bleedx-0.5*lw, top-offy, left+float64(x+1)*w-bleedx-0.5*lw, top-offy-markh)
				l.SetLineWidth(lw)

				crosb.Draw(l)
			}
		}
		l := c.NewLine(left-offx, top+float64(y)*h+bleedy+0.5*lw, left-offx-markw, top+float64(y)*h+bleedy+0.5*lw)
		l.SetLineWidth(lw)
		crosb.Draw(l)
		l = c.NewLine(left-offx, top+float64(y+1)*h-bleedy+0.5*lw, left-offx-markw, top+float64(y+1)*h-bleedy+0.5*lw)
		l.SetLineWidth(lw)
		crosb.Draw(l)
	}

	// use the half of cropmarks block created and a rotated duplicate of it
	// to get a fully cropmarks block
	cros2b := creator.NewBlock(cros2bw, cros2bh)
	cros2b.SetPos(0.0, 0.0)
	cros2b.Draw(crosb)
	crosb.SetAngle(-180)
	crosb.SetPos(cros2bw-(right-left), cros2bh-(bottom-top))
	cros2b.Draw(crosb)
	crosb.SetAngle(0)
	crosb.SetPos(0.0, 0.0)

	// start imposition
	var (
		pg   *pdf.PdfPage
		bk   *creator.Block
		i, j int
	)
grid:
	for {
		for y := 0; y < row; y++ {
			for x := 0; x < len(ff); x++ {
				if i >= np {
					break grid
				}
				// take flow order into account
				num := ff[x] + j*col
				// num resulted larger than number of pages
				// place an empty space with the right wide
				if num > np {
					xpos += float64(w)
					continue
				}
				// get the page number from pages slice
				num = pxp[num-1]

				// check the need for a new page
				if i >= maxOnPage {
					nextPage = (maxOnPage+i)%maxOnPage == 0
				}
				if nextPage {
					// put cropmarks on sheet
					c.Draw(cros2b)
					// initialize position
					ypos = top
					c.NewPage()
					nextPage = false
				}

				// count pages processed
				i++

				// what just a certain page?
				if samepage > 0 {
					num = samepage
				}

				// import page
				pg, err = pdfReader.GetPage(num)
				if err != nil {
					log.Fatal(err)
				}
				bk, err = creator.NewBlockFromPage(pg)
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
				bk.SetPos(xposx, yposy)
				_ = c.Draw(bk)

				xpos += float64(w)
			}
			ypos += float64(h)
			xpos = left
			j++
		}
		// the poor man's vizual indicator that something is happening
		fmt.Print(".")
	}
	// put cropmarks for the last sheet
	c.Draw(cros2b)

	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)

	log.Printf("time taken %v\n", elapsed)
	log.Printf("file %s writteni\n", fout)
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
