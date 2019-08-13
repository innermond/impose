package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/unidoc/unipdf/v3/creator"
)

var (
	fn                       string
	fout                     string
	width                    float64
	height                   float64
	autopage                 bool
	autopadding              float64
	unit                     string
	top, left, bottom, right float64
	center, centerx, centery bool
	pages                    string
	postfix                  string
	samepage                 int
	repeat                   bool
	grid                     string
	flow                     string
	angle                    float64
	bleed, bleedx, bleedy    float64
	offset, offx, offy       float64
	marksize, markw, markh   float64
	showcropmark             bool
	bookletMode              bool
	creep                    float64
	outline                  bool
)

var (
	fileFlags = map[string]bool{
		"f":       true,
		"o":       true,
		"postfix": true,
		"unit":    true,
	}
	geometryFlags = map[string]bool{
		"width":       true,
		"height":      true,
		"top":         true,
		"left":        true,
		"bottom":      true,
		"right":       true,
		"autopage":    true,
		"autopadding": true,
	}
	positionFlags = map[string]bool{
		"center":  true,
		"centerx": true,
		"centery": true,
		"angle":   true,
	}
	gridFlags = map[string]bool{
		"grid":  true,
		"flow":  true,
		"pages": true,
	}
	markFlags = map[string]bool{
		"offset":   true,
		"offx":     true,
		"offy":     true,
		"bleed":    true,
		"bleedy":   true,
		"bleedx":   true,
		"marksize": true,
		"markw":    true,
		"markh":    true,
	}
	viewFlags = map[string]bool{
		"nocropmark": true,
		"outline":    true,
	}
)

func initFileFlags(flagset *flag.FlagSet) {
	flagset.StringVar(&unit, "unit", "mm", "unit of measurements")
	flagset.StringVar(&fn, "f", "", "source pdf file")
	flagset.StringVar(&fout, "o", "", "imposition pdf file")
	flagset.StringVar(&postfix, "postfix", "imposition", "final page termination")
}

func initGeometryFlags(flagset *flag.FlagSet) {
	flagset.Float64Var(&width, "width", 320.0, "imposition sheet width")
	flagset.Float64Var(&height, "height", 450.0, "imposition sheet height")
	flagset.Float64Var(&top, "top", 0.0, "top margin")
	flagset.Float64Var(&left, "left", 0.0, "left margin")
	flagset.Float64Var(&bottom, "bottom", 0.0, "bottom margin")
	flagset.Float64Var(&right, "right", 0.0, "right margin")
	flagset.BoolVar(&autopage, "autopage", false, "calculate proper dimensions for imposition sheet")
	flagset.Float64Var(&autopadding, "autopadding", 2.0, "padding arround imposition")
}

func initPositionFlags(flagset *flag.FlagSet) {
	flagset.BoolVar(&center, "center", false, "center along sheet axes")
	flagset.BoolVar(&centerx, "centerx", false, "center along sheet width")
	flagset.BoolVar(&centery, "centery", false, "center along sheet height")
	flagset.Float64Var(&angle, "angle", 0.0, "angle to angle pages")
}

func initGridFlags(flagset *flag.FlagSet) {
	flagset.StringVar(&grid, "grid", "", "imposition layout columns x  rows. ex: 2x3")
	flagset.StringVar(&flow, "flow", "", "it works along with grid flag. how pages are ordered on every row, they are flowing from 1 to col, but that can be changed, ex: 4,2,1,3")
	flagset.StringVar(&pages, "pages", "", "pages requested by imposition")
}

func initMarkFlags(flagset *flag.FlagSet) {
	flagset.Float64Var(&offset, "offset", 2.0, "distance cut mark keeps from the last edge")
	flagset.Float64Var(&offx, "offx", 2.0, " axe x distance cut mark keeps from the last edge")
	flagset.Float64Var(&offy, "offy", 2.0, " axe y distance cut mark keeps from the last edge")
	flagset.Float64Var(&bleed, "bleed", 0.0, "distance cut mark has been given in respect to the last edge")
	flagset.Float64Var(&bleedx, "bleedx", 0.0, "axe x distance cut mark has been given in respect to the last edge")
	flagset.Float64Var(&bleedy, "bleedy", 0.0, "axe y distance cut mark has been given in respect to the last edge")
	flagset.Float64Var(&marksize, "marksize", 5.0, "cut mark size")
	flagset.Float64Var(&markw, "markw", 5.0, "axe x cut mark size")
	flagset.Float64Var(&markh, "markh", 5.0, "axe y cut mark size")
}

func initViewFlags(flagset *flag.FlagSet) {
	flagset.BoolVar(&showcropmark, "nocropmark", true, "output will not have cropmarks")
	flagset.BoolVar(&outline, "outline", false, "draw a containing rect around imported page")
}

func commonFlags() map[string]bool {
	out := map[string]bool{}
	fff := []map[string]bool{fileFlags, geometryFlags, positionFlags, gridFlags, markFlags, viewFlags}
	for _, ff := range fff {
		for flagname, val := range ff {
			out[flagname] = val
		}
	}
	return out
}

func param() error {
	var (
		err     error
		flagset = flag.NewFlagSet("cli", flag.ExitOnError)
		same    = []string{}
		spec    = []string{}
	)

	flagset.Usage = func() {
		fmt.Printf(`  -repeat command
	repeat every requested page on imposition sheet in respect to grid
  -samepage <numpage> command
	impose chosen samepage using grid by pages number times 
`)
		flagset.PrintDefaults()
	}

	var (
		usage bool
		cmd   = os.Args[1]
	)

	// start to initialize flags definition
	switch cmd {
	case "repeat":
		// ./impose repeat ...
		same, spec = clivide(os.Args[2:], commonFlags())
		repeat = true
		// setup specifi flags then
		// parse specific flag if any
		flagset.Parse(spec)
	case "samepage":
		// ./impose samepage <pagenum> ...
		same, spec = clivide(os.Args[3:], commonFlags())
		samepage, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		if samepage < 1 {
			usage = true
		}
		flagset.Parse(spec)
	case "booklet":
		// ./impose booklet -creep
		same, spec = clivide(os.Args[2:], commonFlags())
		bookletMode = true
		// specific flag
		flagset.Float64Var(&creep, "creep", 0.0, "adjust imposition to deal with sheet's tickness")
		flagset.Parse(spec)
	default:
		same, spec = clivide(os.Args[1:], commonFlags())
		flagset.Parse(spec)
		if !isFlag(cmd) {
			usage = true
		}
	}

	// set common flags
	initFileFlags(flagset)
	initGeometryFlags(flagset)
	initPositionFlags(flagset)
	initGridFlags(flagset)
	initMarkFlags(flagset)
	initViewFlags(flagset)
	// end flags definition
	flagset.Parse(same)

	if usage {
		flagset.Usage()
		os.Exit(1)
	}

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
	autopadding *= creator.PPMM

	flagset.Visit(func(f *flag.Flag) {
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
		case "bookletMode":
			bookletMode = true
			creep *= creator.PPMM
			if creep > bleedx {
				creep = bleedx
			}
		case "nocropmark":
			showcropmark = false
		case "autopage":
			autopage = true
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

	if !bookletMode {
		creep = 0.0
	} else {
		repeat = false
	}

	if !autopage {
		autopadding = 0.0
	}

	return err
}
