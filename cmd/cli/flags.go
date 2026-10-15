package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/innermond/impose"
	"github.com/unidoc/unipdf/v3/creator"
)

var (
	fn     string
	width  float64
	height float64

	autopage    bool
	autopadding float64

	unit string

	top, left, bottom, right float64

	center, centerx, centery bool

	pages                  string
	grid                   string
	repeat                 bool
	clone                  string
	clonepadx              float64
	clonepady              float64
	flow                   string
	duplex                 bool
	flip                   bool
	reverse                bool
	turn                   float64
	weld                   int
	angle                  float64
	bleed, bleedx, bleedy  float64
	offset, offx, offy     float64
	marksize, markw, markh float64
	showcropmark           bool
	nocropmark             bool
	showcropmarkPages      markPages
	bookletMode            bool
	creep                  float64
	outline                bool

	memprofile string
	cpuprofile string
	verbosity  int
)

var (
	fileFlags = map[string]bool{
		"f":         true,
		"o":         true,
		"unit":      true,
		"verbosity": true,
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
		"grid":      true,
		"clone":     true,
		"clonepadx": true,
		"clonepady": true,
		"flow":      true,
		"pages":     true,
		"duplex":    true,
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
		"nocropmark":    true,
		"cropmarkPages": true,
		"outline":       true,
	}
	duplexFlags = map[string]bool{
		"flip":    true,
		"turn":    true,
		"reverse": true,
		"duplex":  true,
		"weld":    true,
	}
	debugFlags = map[string]bool{
		"mem": true,
		"cpu": true,
	}
)

func initFileFlags(flagset *flag.FlagSet) {
	flagset.StringVar(&unit, "unit", "mm", "unit of lengths given in the command line: mm, cm, in, pt")
	flagset.StringVar(&fn, "f", "", "source pdf file")
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
	if gridFlags["grid"] {
		flagset.StringVar(&grid, "grid", "", "imposition layout columns x  rows. ex: 2x3")
	}
	if gridFlags["clone"] {
		flagset.StringVar(&clone, "clone", "1x1", "clone the groups")
	}
	// the pads are needed even when clone is not a flag, as in repeat mode
	flagset.Float64Var(&clonepadx, "clonepadx", 0.0, "pad distance the clones respect along x axe")
	flagset.Float64Var(&clonepady, "clonepady", 0.0, "pad distance the clones respect along y axe")
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
	flagset.BoolVar(&nocropmark, "nocropmark", false, "output will not have cropmarks")
	flagset.Var(&showcropmarkPages, "cropmarkPages", "output will have cropmarks only on target pages - 1 first page, 2 second ans so on")
	flagset.BoolVar(&outline, "outline", false, "draw a containing rect around imported page")
}
func initDebugFlags(flagset *flag.FlagSet) {
	flagset.StringVar(&memprofile, "mem", "", "memory profiling")
	flagset.StringVar(&cpuprofile, "cpu", "", "cpu profiling")
	flagset.IntVar(&verbosity, "verbosity", 0, "data report amount")
}

func commonFlags() map[string]bool {
	out := map[string]bool{}
	fff := []map[string]bool{fileFlags, geometryFlags, positionFlags, gridFlags, markFlags, viewFlags, duplexFlags, debugFlags}
	for _, ff := range fff {
		for flagname, val := range ff {
			out[flagname] = val
		}
	}
	return out
}

func initFlagDuplex(flagset *flag.FlagSet) {
	flagset.Float64Var(&turn, "turn", 0.0, "rotate page with specified angle")
	flagset.BoolVar(&flip, "flip", false, "flip grouped pages")
	flagset.BoolVar(&reverse, "reverse", false, "reverse order of pages")
	flagset.StringVar(&flow, "flow", "", "order and repetition of pages inside every block of max(flow) pages, numbered from 1. ex: 1,1,1,2,2,2")
	flagset.IntVar(&weld, "weld", 1, "length of pages group")
	flagset.BoolVar(&duplex, "duplex", false, "activate duplex")
}

func param() error {
	// set common flags
	if len(os.Args) == 1 {
		return &impose.Error{Code: impose.EINVALID, Message: "no arguments provided"}
	}

	var (
		err error

		flagset = flag.NewFlagSet("cli", flag.ExitOnError)
		cmd     = os.Args[1]
		same    = []string{}
		spec    = []string{}

		usage   func()
		usagefn = func(msg string) func() {
			return func() {
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				flagset.Usage()
				os.Exit(1)
			}
		}
	)
	// start to initialize flags definition
	switch cmd {
	case "repeat":
		// ./impose repeat 3x2 ...
		if len(os.Args) < 3 || isFlag(os.Args[2]) {
			return &impose.Error{Code: impose.EINVALID, Message: "repeat needs a grid, ex: repeat 3x2"}
		}
		grid = "1x1"
		gridFlags["grid"] = false
		clone = os.Args[2]
		gridFlags["clone"] = false
		same, spec = clivide(os.Args[3:], commonFlags())
		repeat = true
		// setup specific flags then
		// parse specific flag if any
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
			usage = usagefn("not defined")
		}
	}
	log.Println("init flags...")
	initFileFlags(flagset)
	initGeometryFlags(flagset)
	initPositionFlags(flagset)
	initGridFlags(flagset)
	initMarkFlags(flagset)
	initViewFlags(flagset)
	initFlagDuplex(flagset)
	initDebugFlags(flagset)
	// end common flags definition
	flagset.Usage = func() {
		fmt.Fprint(os.Stderr, `  -repeat command
	repeat every requested page on imposition sheet in respect to grid
  -booklet command
  	booklet impose using a flow 4-1 2-3 dedicated for booklet
	it has its own flag -creep
`)
		flagset.PrintDefaults()
	}

	flagset.Parse(same)
	if usage != nil {
		usage()
	}

	if fn == "" {
		return errors.New("pdf file required")
	}

	// flags that were given in the command line, the rest keep their defaults
	seen := map[string]bool{}
	flagset.Visit(func(f *flag.Flag) { seen[f.Name] = true })

	// all lengths to points; defaults are in millimetres, values given by user are in -unit
	ppu, err := unitFactor(unit)
	if err != nil {
		return &impose.Error{Code: impose.EINVALID, Message: err.Error()}
	}
	lengths := map[string]*float64{
		"clonepadx": &clonepadx, "clonepady": &clonepady,
		"left": &left, "right": &right, "top": &top, "bottom": &bottom,
		"width": &width, "height": &height,
		"offset": &offset, "offx": &offx, "offy": &offy,
		"bleed": &bleed, "bleedx": &bleedx, "bleedy": &bleedy,
		"marksize": &marksize, "markw": &markw, "markh": &markh,
		"autopadding": &autopadding, "creep": &creep,
	}
	for name, length := range lengths {
		if seen[name] {
			*length *= ppu
		} else {
			*length *= creator.PPMM
		}
	}

	// a general flag gives its value to the specific ones, unless these were given too
	if seen["offset"] {
		if !seen["offx"] {
			offx = offset
		}
		if !seen["offy"] {
			offy = offset
		}
	}
	if seen["marksize"] {
		if !seen["markw"] {
			markw = marksize
		}
		if !seen["markh"] {
			markh = marksize
		}
	}
	if seen["bleed"] {
		if !seen["bleedx"] {
			bleedx = bleed
		}
		if !seen["bleedy"] {
			bleedy = bleed
		}
	}
	centerx = centerx || center
	centery = centery || center
	showcropmark = !nocropmark

	// creep cannot exceed the bleed
	if bookletMode && creep > bleedx {
		creep = bleedx
	}
	// last edge is further inside mediabox by bleed amount
	// adjust offsets otherwise they will be aware only by media edge
	offx -= bleedx
	offy -= bleedy

	if !autopage {
		autopadding = 0.0
	}

	return err
}

// markPages knows what pages will show cropmarks
type markPages []int

func (mp *markPages) String() string {
	return fmt.Sprint(*mp)
}

func (mp *markPages) Set(v string) error {
	// set it once
	if len(*mp) > 0 {
		return errors.New("cropmark pages already set")
	}
	for _, s := range strings.Split(v, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return err
		}
		*mp = append(*mp, n-1)
	}
	return nil
}

// unitFactor gives how many points a length unit has
func unitFactor(u string) (float64, error) {
	switch strings.ToLower(u) {
	case "mm":
		return creator.PPMM, nil
	case "cm":
		return 10 * creator.PPMM, nil
	case "in", "inch":
		return 72, nil
	case "pt":
		return 1, nil
	}
	return 0, fmt.Errorf("unknown unit %q, use mm, cm, in or pt", u)
}
