package impose

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/unidoc/unipdf/v3/creator"
)

// CLIArgs gives the command line arguments, with lengths in millimetres, that make
// the same imposition of file as Options does. Defaults of the command line are left out.
func (o Options) CLIArgs(file string) []string {
	def := DefaultOptions()
	var a []string
	add := func(name string, values ...string) {
		a = append(a, "-"+name)
		a = append(a, values...)
	}
	// mm gives a length in points as millimetres
	mm := func(v float64) string {
		return strconv.FormatFloat(math.Round(v/creator.PPMM*1e4)/1e4, 'f', -1, 64)
	}
	// length adds a flag when its value is not the default
	length := func(name string, v, d float64) {
		if math.Abs(v-d) > 1e-9 {
			add(name, mm(v))
		}
	}
	grid := func(col, row int) string { return fmt.Sprintf("%dx%d", max(col, 1), max(row, 1)) }
	clones := o.CloneX > 1 || o.CloneY > 1

	switch o.Mode {
	case ModeRepeat:
		a = append(a, "repeat", grid(o.CloneX, o.CloneY))
	case ModeBooklet:
		a = append(a, "booklet")
	}
	add("f", file)
	if o.Mode != ModeRepeat {
		if o.Col != 0 || o.Row != 0 {
			add("grid", grid(o.Col, o.Row))
		}
		if clones {
			add("clone", grid(o.CloneX, o.CloneY))
		}
	}
	if o.ClonePadX != 0 {
		add("clonepadx", mm(o.ClonePadX))
	}
	if o.ClonePadY != 0 {
		add("clonepady", mm(o.ClonePadY))
	}
	if o.Pages != "" {
		add("pages", o.Pages)
	}
	if o.Mode == ModeGrid {
		if len(o.Flow) > 0 && !(len(o.Flow) == 1 && o.Flow[0] == 1) {
			ff := make([]string, len(o.Flow))
			for i, v := range o.Flow {
				ff[i] = strconv.Itoa(v)
			}
			add("flow", strings.Join(ff, ","))
		}
		if o.Duplex {
			add("duplex")
			if o.Weld > 1 {
				add("weld", strconv.Itoa(o.Weld))
			}
		}
	}
	if o.Mode != ModeRepeat {
		if o.Flip {
			add("flip")
		}
		if o.Reverse {
			add("reverse")
		}
	}
	if o.Turn != 0 {
		add("turn", strconv.FormatFloat(o.Turn, 'f', -1, 64))
	}
	if o.Mode == ModeBooklet && o.Creep != 0 {
		add("creep", mm(o.Creep))
	}
	if o.Mode == ModeBooklet && o.PadBlank {
		add("padblank")
	}

	if o.Autopage {
		add("autopage")
		length("autopadding", o.Autopadding, def.Autopadding)
	} else {
		length("width", o.Width, def.Width)
		length("height", o.Height, def.Height)
	}
	length("left", o.Left, 0)
	length("right", o.Right, 0)
	length("top", o.Top, 0)
	length("bottom", o.Bottom, 0)
	switch {
	case o.CenterX && o.CenterY:
		add("center")
	case o.CenterX:
		add("centerx")
	case o.CenterY:
		add("centery")
	}
	if o.Angle != 0 {
		add("angle", strconv.FormatFloat(o.Angle, 'f', -1, 64))
	}

	// bleed, offset and mark size are given together when both axes are the same
	pair := func(both, x, y string, vx, vy, d float64) {
		switch {
		case vx == vy:
			length(both, vx, d)
		default:
			length(x, vx, d)
			length(y, vy, d)
		}
	}
	pair("bleed", "bleedx", "bleedy", o.Bleedx, o.Bleedy, 0)
	// the room cropmarks need counts for the size of the sheet, even if they are not drawn
	pair("offset", "offx", "offy", o.Offx, o.Offy, def.Offx)
	pair("marksize", "markw", "markh", o.Markw, o.Markh, def.Markw)
	if o.NoCropmark {
		add("nocropmark")
	} else if len(o.CropmarkPages) > 0 {
		ss := make([]string, len(o.CropmarkPages))
		for i, v := range o.CropmarkPages {
			ss[i] = strconv.Itoa(v + 1)
		}
		add("cropmarkPages", strings.Join(ss, ","))
	}
	if o.Outline {
		add("outline")
	}
	return a
}
