package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/innermond/impose"
)

// Modes of the imposition, as they are shown.
const (
	modeGrid    = "Grid"
	modeRepeat  = "Repeat"
	modeBooklet = "Booklet"
)

// Settings are the values of the form, as the user typed them: numbers are text so that a half
// typed value is not lost, lengths are in Unit.
type Settings struct {
	Mode string
	Unit string

	Width, Height              string
	Autopage                   bool
	Autopadding                string
	Top, Right, Bottom, Left   string
	CenterX, CenterY           bool
	Angle                      string
	Cols, Rows                 string
	CloneX, CloneY, PadX, PadY string
	Pages, Flow                string
	Duplex, Flip, Reverse      bool
	PadBlank                   bool
	Weld, Turn, Creep          string
	Marks                      bool
	Bleed, Offset, MarkSize    string
	MarkPages                  string
	Outline                    bool
}

// lengthFields are the settings that hold a length, they follow the unit.
func (s *Settings) lengthFields() []*string {
	return []*string{
		&s.Width, &s.Height, &s.Autopadding,
		&s.Top, &s.Right, &s.Bottom, &s.Left,
		&s.PadX, &s.PadY, &s.Creep,
		&s.Bleed, &s.Offset, &s.MarkSize,
	}
}

// defaultSettings are the defaults of the command line.
func defaultSettings() Settings {
	return Settings{
		Mode: modeGrid, Unit: "mm",
		Width: "320", Height: "450", Autopadding: "2",
		CloneX: "1", CloneY: "1", Weld: "1",
		Marks: true, Offset: "2", MarkSize: "5",
	}
}

// formatNumber writes a number the shortest way: without noise from floating point, and a value
// that is within rounding of a number with 2 decimals is that number (31.99994 cm is 32).
func formatNumber(v float64) string {
	if r := roundTo(v, 2); math.Abs(v-r) < 2e-4 {
		v = r
	}
	return strconv.FormatFloat(roundTo(v, 4), 'f', -1, 64)
}

func roundTo(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// convertUnit changes the unit, lengths keep their size: 10 mm become 1 cm.
// A length that is not a number is left as it is.
func (s *Settings) convertUnit(unit string) error {
	from, err := impose.UnitPoints(s.Unit)
	if err != nil {
		return err
	}
	to, err := impose.UnitPoints(unit)
	if err != nil {
		return err
	}
	for _, f := range s.lengthFields() {
		v, err := strconv.ParseFloat(strings.TrimSpace(*f), 64)
		if err != nil {
			continue
		}
		*f = formatNumber(v * from / to)
	}
	s.Unit = unit
	return nil
}

// parser collects the first error of a series of conversions
type parser struct {
	err  error
	unit float64
}

func (p *parser) fail(name, what string) {
	if p.err == nil {
		p.err = fmt.Errorf("%s: %s", name, what)
	}
}

// float reads an optional number, empty is 0
func (p *parser) float(name, v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
	if err != nil {
		p.fail(name, fmt.Sprintf("%q is not a number", v))
	}
	return f
}

// length reads an optional length and gives it in points
func (p *parser) length(name, v string) float64 {
	f := p.float(name, v)
	if f < 0 {
		p.fail(name, "can not be negative")
	}
	return f * p.unit
}

// count reads an optional whole number, empty is def
func (p *parser) count(name, v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		p.fail(name, fmt.Sprintf("%q is not a whole number, 1 or more", v))
	}
	return n
}

// ints reads a comma separated list of numbers, each one 1 or more
func (p *parser) ints(name, v string) []int {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(v, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 {
			p.fail(name, fmt.Sprintf("%q is not a page number, they start from 1", strings.TrimSpace(part)))
			return nil
		}
		out = append(out, n)
	}
	return out
}

// options gives what the settings ask for, or what is wrong with them.
func (s Settings) options() (impose.Options, error) {
	unit, err := impose.UnitPoints(s.Unit)
	if err != nil {
		return impose.Options{}, err
	}
	p := &parser{unit: unit}
	o := impose.Options{}

	switch s.Mode {
	case modeRepeat:
		o.Mode = impose.ModeRepeat
	case modeBooklet:
		o.Mode = impose.ModeBooklet
	default:
		o.Mode = impose.ModeGrid
	}

	o.Width, o.Height = p.length("width", s.Width), p.length("height", s.Height)
	o.Top, o.Right, o.Bottom, o.Left = p.length("top", s.Top), p.length("right", s.Right), p.length("bottom", s.Bottom), p.length("left", s.Left)
	o.Autopage, o.Autopadding = s.Autopage, p.length("padding", s.Autopadding)
	o.CenterX, o.CenterY = s.CenterX, s.CenterY
	o.Angle = p.float("angle", s.Angle)

	// the grid is given by both numbers or by none
	col, row := strings.TrimSpace(s.Cols), strings.TrimSpace(s.Rows)
	if o.Mode != impose.ModeRepeat {
		switch {
		case col == "" && row == "":
		case col == "" || row == "":
			p.fail("grid", "give both columns and rows, or none")
		default:
			o.Col, o.Row = p.count("columns", col, 0), p.count("rows", row, 0)
		}
	}
	o.CloneX, o.CloneY = p.count("clones along x", s.CloneX, 1), p.count("clones along y", s.CloneY, 1)
	o.ClonePadX, o.ClonePadY = p.length("clone pad x", s.PadX), p.length("clone pad y", s.PadY)

	o.Pages = strings.TrimSpace(s.Pages)
	if o.Mode == impose.ModeGrid {
		o.Flow = p.ints("flow", s.Flow)
		o.Duplex = s.Duplex
		o.Weld = p.count("weld", s.Weld, 1)
	}
	if o.Mode != impose.ModeRepeat {
		o.Flip, o.Reverse = s.Flip, s.Reverse
	}
	// turn is shown only in booklet, in the other modes it is the same as the angle
	if o.Mode == impose.ModeBooklet {
		o.Turn = p.float("turn", s.Turn)
		o.PadBlank = s.PadBlank
	}

	bleed := p.length("bleed", s.Bleed)
	o.Bleedx, o.Bleedy = bleed, bleed
	o.Offx, o.Offy = p.length("offset", s.Offset), p.length("offset", s.Offset)
	o.Markw, o.Markh = p.length("mark size", s.MarkSize), p.length("mark size", s.MarkSize)
	o.NoCropmark = !s.Marks
	for _, n := range p.ints("marked sheets", s.MarkPages) {
		o.CropmarkPages = append(o.CropmarkPages, n-1)
	}
	o.Outline = s.Outline

	if o.Mode == impose.ModeBooklet {
		o.Creep = p.length("creep", s.Creep)
		// creep cannot exceed the bleed
		o.Creep = min(o.Creep, o.Bleedx)
	}
	if p.err != nil {
		return impose.Options{}, p.err
	}
	return o, nil
}

// command gives the command line that makes the same imposition
func (s Settings) command(file string) (string, error) {
	o, err := s.options()
	if err != nil {
		return "", err
	}
	args := o.CLIArgs(file)
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\"'$&;<>|*?()") {
			args[i] = strconv.Quote(a)
		}
	}
	return "impose " + strings.Join(args, " "), nil
}

// store is what stays between runs: the last settings and the presets that were saved.
type store struct {
	Current Settings
	Presets map[string]Settings
	// Dir is the folder last used to open a pdf.
	Dir string
	// SearchDir is the folder last searched in.
	SearchDir string
	// Recent are the pdf files opened, the last one first.
	Recent []string
	// HideThumbs shows boxes in place of the pages in the preview.
	HideThumbs bool
	// SeeThrough is how the back of the sheet shows through it in the preview: seeNone, seeSide or seeHead.
	SeeThrough int
}

// maxRecent is how many files are remembered.
const maxRecent = 10

// remember puts a file first in the recent files.
func (st *store) remember(path string) {
	out := []string{path}
	for _, p := range st.Recent {
		if p != path {
			out = append(out, p)
		}
	}
	if len(out) > maxRecent {
		out = out[:maxRecent]
	}
	st.Recent = out
}

// forget takes a file out of the recent files.
func (st *store) forget(path string) {
	var out []string
	for _, p := range st.Recent {
		if p != path {
			out = append(out, p)
		}
	}
	st.Recent = out
}

// cleanPath makes a path of what was typed, pasted or dropped: quotes around it, a file:// address and
// a leading ~ are taken care of. Empty gives empty.
func cleanPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	if s == "" {
		return ""
	}
	if len(s) > 7 && strings.EqualFold(s[:7], "file://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Path
			// file:///C:/dir/a.pdf gives /C:/dir/a.pdf
			if len(s) > 2 && s[0] == '/' && s[2] == ':' {
				s = s[1:]
			}
			s = filepath.FromSlash(s)
		}
	}
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[1:])
		}
	}
	return filepath.Clean(s)
}

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "impose", "gui.json"), nil
}

// loadStore reads what was saved, defaults if there is nothing or it can not be read.
func loadStore(path string) store {
	st := store{Current: defaultSettings(), Presets: map[string]Settings{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	var saved store
	if json.Unmarshal(b, &saved) != nil {
		return st
	}
	if saved.Presets == nil {
		saved.Presets = map[string]Settings{}
	}
	if _, err := impose.UnitPoints(saved.Current.Unit); err != nil {
		saved.Current = defaultSettings()
	}
	return saved
}

func (st store) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	// written aside, then moved, so a crash does not leave half a file
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
