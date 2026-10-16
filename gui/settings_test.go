package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/innermond/impose"
	"github.com/unidoc/unipdf/v3/creator"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestDefaultsAreTheCommandLineDefaults(t *testing.T) {
	s := defaultSettings()
	s.Cols, s.Rows = "2", "1"
	o, err := s.options()
	if err != nil {
		t.Fatal(err)
	}
	want := impose.DefaultOptions()
	want.Col, want.Row = 2, 1
	// the default options leave no flow and no clones set
	want.CloneX, want.CloneY = 1, 1
	if !reflect.DeepEqual(o, want) {
		t.Errorf("got %+v\nwant %+v", o, want)
	}
	cmd, err := s.command("in.pdf")
	if err != nil || cmd != "impose -f in.pdf -grid 2x1" {
		t.Errorf("command %q %v", cmd, err)
	}
}

func TestOptionsModes(t *testing.T) {
	s := defaultSettings()
	s.Mode, s.CloneX, s.CloneY, s.PadX, s.PadY = modeRepeat, "3", "2", "5", "7"
	s.Cols, s.Rows = "9", "9" // grid does not apply to repeat
	o, err := s.options()
	if err != nil {
		t.Fatal(err)
	}
	if o.Mode != impose.ModeRepeat || o.Col != 0 || o.CloneX != 3 || o.CloneY != 2 || !near(o.ClonePadX, 5*creator.PPMM) || !near(o.ClonePadY, 7*creator.PPMM) {
		t.Errorf("repeat: %+v", o)
	}

	s = defaultSettings()
	s.Mode, s.Creep, s.Bleed = modeBooklet, "3", "1"
	o, err = s.options()
	if err != nil {
		t.Fatal(err)
	}
	if o.Mode != impose.ModeBooklet || !near(o.Creep, 1*creator.PPMM) {
		t.Errorf("creep should be limited by bleed: %+v", o.Creep)
	}

	s = defaultSettings()
	s.PadBlank = true
	if o, _ = s.options(); o.PadBlank {
		t.Error("blank pages are padded only in booklet")
	}
	s.Mode = modeBooklet
	if o, _ = s.options(); !o.PadBlank {
		t.Error("booklet should pad with blanks")
	}

	s = defaultSettings()
	s.Flow, s.Duplex, s.Weld, s.Cols, s.Rows = "1, 1,2", true, "2", "4", "1"
	s.MarkPages, s.Marks = "1,3", true
	o, err = s.options()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o.Flow, []int{1, 1, 2}) || !o.Duplex || o.Weld != 2 || !reflect.DeepEqual(o.CropmarkPages, []int{0, 2}) {
		t.Errorf("grid: %+v", o)
	}
}

func TestHiddenFieldsAreIgnored(t *testing.T) {
	s := defaultSettings()
	s.Turn, s.Creep, s.Flow, s.Weld, s.Duplex, s.Bleed = "180", "1", "1,2", "2", true, "1"
	s.Cols, s.Rows = "2", "1"
	// turn and creep are for the booklet
	o, err := s.options()
	if err != nil || o.Turn != 0 || o.Creep != 0 {
		t.Errorf("grid: turn %v creep %v %v", o.Turn, o.Creep, err)
	}
	s.Mode = modeRepeat
	if o, _ = s.options(); o.Turn != 0 || len(o.Flow) != 0 || o.Duplex || o.Col != 0 {
		t.Errorf("repeat: %+v", o)
	}
	s.Mode = modeBooklet
	if o, _ = s.options(); o.Turn != 180 || o.Creep == 0 || len(o.Flow) != 0 || o.Duplex {
		t.Errorf("booklet: %+v", o)
	}
}

func TestOptionsErrors(t *testing.T) {
	tt := []struct {
		name string
		set  func(s *Settings)
		msg  string
	}{
		{"width text", func(s *Settings) { s.Width = "wide" }, "width"},
		{"negative", func(s *Settings) { s.Left = "-3" }, "left: can not be negative"},
		{"half grid", func(s *Settings) { s.Cols = "2" }, "grid: give both"},
		{"grid zero", func(s *Settings) { s.Cols, s.Rows = "0", "1" }, "columns"},
		{"flow zero", func(s *Settings) { s.Flow = "1,0" }, "flow"},
		{"flow text", func(s *Settings) { s.Flow = "a" }, "flow"},
		{"unit", func(s *Settings) { s.Unit = "furlong" }, "unknown unit"},
		{"marks", func(s *Settings) { s.MarkPages = "0" }, "marked sheets"},
	}
	for _, tc := range tt {
		s := defaultSettings()
		tc.set(&s)
		if _, err := s.options(); err == nil || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%s: got %v want %q", tc.name, err, tc.msg)
		}
	}
	// decimal comma is accepted
	s := defaultSettings()
	s.Width = "320,5"
	if o, err := s.options(); err != nil || !near(o.Width, 320.5*creator.PPMM) {
		t.Errorf("comma: %v %v", o.Width, err)
	}
}

func TestConvertUnit(t *testing.T) {
	s := defaultSettings()
	s.Left = "oops"
	if err := s.convertUnit("in"); err != nil {
		t.Fatal(err)
	}
	if s.Unit != "in" || s.Width != "12.5984" || s.Left != "oops" {
		t.Errorf("to inch: %q %q %q", s.Unit, s.Width, s.Left)
	}
	// physical size does not change
	s.Left = ""
	o1, err := s.options()
	if err != nil {
		t.Fatal(err)
	}
	d := defaultSettings()
	o2, _ := d.options()
	if math.Abs(o1.Width-o2.Width) > 0.01 || math.Abs(o1.Offx-o2.Offx) > 0.01 {
		t.Errorf("size changed with the unit: %v %v / %v %v", o1.Width, o2.Width, o1.Offx, o2.Offx)
	}
	if err := s.convertUnit("cm"); err != nil || s.Width != "32" || s.Height != "45" {
		t.Errorf("to cm: %q %q %v", s.Width, s.Height, err)
	}
	if err := s.convertUnit("furlong"); err == nil || s.Unit != "cm" {
		t.Errorf("bad unit must change nothing: %v %q", err, s.Unit)
	}
}

func TestCommandQuoting(t *testing.T) {
	s := defaultSettings()
	s.Cols, s.Rows = "2", "1"
	cmd, err := s.command("/home/me/my file.pdf")
	if err != nil || !strings.Contains(cmd, `-f "/home/me/my file.pdf"`) {
		t.Errorf("%q %v", cmd, err)
	}
	s.Mode, s.CloneX, s.CloneY = modeRepeat, "3", "2"
	cmd, _ = s.command("a.pdf")
	if !strings.HasPrefix(cmd, "impose repeat 3x2 -f a.pdf") {
		t.Errorf("%q", cmd)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "gui.json")
	// nothing saved: defaults
	st := loadStore(path)
	if !reflect.DeepEqual(st.Current, defaultSettings()) || st.Presets == nil {
		t.Fatalf("defaults: %+v", st)
	}
	st.Current.Width = "123"
	st.Presets["A3 booklet"] = Settings{Mode: modeBooklet, Unit: "mm", Width: "420"}
	st.Dir = "/tmp"
	if err := st.save(path); err != nil {
		t.Fatal(err)
	}
	got := loadStore(path)
	if got.Current.Width != "123" || got.Presets["A3 booklet"].Width != "420" || got.Dir != "/tmp" {
		t.Errorf("not kept: %+v", got)
	}
	// a damaged file or an unknown unit does not stop the program
	if err := writeFile(path, "{not json"); err != nil {
		t.Fatal(err)
	}
	if got := loadStore(path); !reflect.DeepEqual(got.Current, defaultSettings()) {
		t.Errorf("damaged: %+v", got.Current)
	}
	if err := writeFile(path, `{"Current":{"Unit":"furlong"}}`); err != nil {
		t.Fatal(err)
	}
	if got := loadStore(path); got.Current.Unit != "mm" {
		t.Errorf("unit: %+v", got.Current)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestCleanPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home")
	}
	tt := map[string]string{
		"":                          "",
		"   ":                       "",
		"/tmp/a.pdf":                "/tmp/a.pdf",
		`  "/tmp/my file.pdf"  `:    "/tmp/my file.pdf",
		`'/tmp/a.pdf'`:              "/tmp/a.pdf",
		"file:///tmp/my%20file.pdf": "/tmp/my file.pdf",
		"FILE:///tmp/a.pdf":         "/tmp/a.pdf",
		"/tmp/../tmp//a.pdf":        "/tmp/a.pdf",
		"~/docs/a.pdf":              filepath.Join(home, "docs", "a.pdf"),
		"~":                         home,
		"docs/a.pdf":                "docs/a.pdf",
		"~other/a.pdf":              "~other/a.pdf",
	}
	for in, want := range tt {
		if got := cleanPath(in); got != want {
			t.Errorf("cleanPath(%q) = %q want %q", in, got, want)
		}
	}
}

func TestRecentFiles(t *testing.T) {
	var st store
	for _, p := range []string{"a", "b", "c"} {
		st.remember(p)
	}
	st.remember("a")
	if !reflect.DeepEqual(st.Recent, []string{"a", "c", "b"}) {
		t.Errorf("moved to front, no repeat: %v", st.Recent)
	}
	for i := 0; i < 20; i++ {
		st.remember(string(rune('a'+i)) + ".pdf")
	}
	if len(st.Recent) != maxRecent || st.Recent[0] != "t.pdf" {
		t.Errorf("capped: %v", st.Recent)
	}
	st.forget("t.pdf")
	st.forget("not there")
	if len(st.Recent) != maxRecent-1 || st.Recent[0] != "s.pdf" {
		t.Errorf("forget: %v", st.Recent)
	}

	// kept between runs, and a file without them loads too
	path := filepath.Join(t.TempDir(), "gui.json")
	if err := st.save(path); err != nil {
		t.Fatal(err)
	}
	if got := loadStore(path); !reflect.DeepEqual(got.Recent, st.Recent) {
		t.Errorf("not kept: %v", got.Recent)
	}
	if err := writeFile(path, `{"Current":{"Unit":"mm","Mode":"Grid"},"Dir":"/tmp"}`); err != nil {
		t.Fatal(err)
	}
	if got := loadStore(path); len(got.Recent) != 0 || got.Dir != "/tmp" {
		t.Errorf("old file: %+v", got)
	}
}
