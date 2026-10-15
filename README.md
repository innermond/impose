# impose

Command line imposition for PDF files: places the pages of a PDF on larger sheets (grids, repeated
pages, duplex, booklets) and adds cropmarks.

```
impose [command] -f input.pdf [flags] > output.pdf
```

## Build

```
go build -o impose ./cmd/cli
```

## Things to know first

- **The PDF is written to stdout.** Always redirect it: `impose ... > out.pdf`. Logs go to stderr.
- **Lengths are in millimetres** (`-width`, `-left`, `-bleed`, `-offset`, ...). `-unit mm|cm|in|pt` changes the unit of
  the lengths you write in the command line; defaults stay in millimetres.
- Sheet size defaults to `320 x 450` mm. Use `-width`/`-height`, or `-autopage` to size the sheet to the content.
- Negative values work as usual: `-angle -90` or `-angle=-90`.
- Page numbers (`-pages`, `-flow`, `-cropmarkPages`) are numbered from 1.
- The `unipdf` library is unlicensed and prints `Unlicensed copy of unidoc...` to stdout before `%PDF`. Most readers
  accept it; get a unidoc license to remove it.

The examples below use a 4-page input `in.pdf` (pages numbered 1-4) and an 8-page `in8.pdf`.
"Sheet 1: 1 3" means the first output sheet carries pages 1 and 3.

## Commands

| Command | What it does |
|---|---|
| *(none)* | Impose pages on a grid, in order, optionally reordered with `-flow` / `-duplex`. |
| `repeat <cols>x<rows>` | Put every page on its own sheet(s), cloned `cols x rows` times. |
| `booklet` | Impose for a folded booklet (last-first / second-second last), duplex ready. |

## Common flags

### Input and sheet

| Flag | Default | Meaning |
|---|---|---|
| `-f file.pdf` | required | Source PDF. All pages are assumed to have the size of page 1. |
| `-pages sel` | all | Pages to use: `1-4`, `1,3`, `2-4,7`. Sorted and merged (no repeats; use `-flow` for repeats). `-2` means `1-2`. |
| `-width`, `-height` | 320, 450 | Sheet size. |
| `-left -right -top -bottom` | 0 | Sheet margins. |
| `-autopage` | off | Compute the sheet size from the grid, clones, page size, margins, cropmark space and `-autopadding`. It only sizes the sheet; add `-center` to put the grid in the middle so the marks fit. |
| `-autopadding` | 2 | Extra space around the imposition when `-autopage` is used. |
| `-center`, `-centerx`, `-centery` | off | Center the grid on the sheet (by adjusting margins). |
| `-angle deg` | 0 | Rotate every page. `90`, `-90`/`270`, `180`, `-180`. Sheet size from `-autopage` accounts for 90/270. |
| `-outline` | off | Draw a rectangle around each placed page. |

### Grid

| Flag | Default | Meaning |
|---|---|---|
| `-grid CxR` | required | Columns x rows per sheet, e.g. `3x2`. |
| `-clone CxR` | `1x1` | Repeat the whole grid `C x R` times on the sheet. |
| `-clonepadx`, `-clonepady` | 0 | Gap between clones. Work in all modes, including `repeat`. |
| `-flow list` | `1` | Order and repetition of pages inside every block of `max(flow)` pages, e.g. `1,1,1,2,2,2`. |

### Cropmarks and bleed

| Flag | Default | Meaning |
|---|---|---|
| `-nocropmark` | cropmarks on | Do not draw cropmarks. |
| `-cropmarkPages list` | all sheets | Draw cropmarks only on these sheets (`1` = first sheet, `1,3`). |
| `-bleed`, `-bleedx`, `-bleedy` | 0 | Bleed. Pages are expanded from their TrimBox (or CropBox) by this amount, never beyond the MediaBox; marks sit on the trim line. |
| `-offset`, `-offx`, `-offy` | 2 | Distance cropmarks keep from the trim edge. |
| `-marksize`, `-markw`, `-markh` | 5 | Cropmark length. |

Cropmarks are drawn *outside* the grid, so leave margins of at least `offset - bleed + marksize` (7 mm by default)
on the sides, or use `-autopage -center`. With zero margins the top and left marks fall off the sheet.

### Duplex

| Flag | Default | Meaning |
|---|---|---|
| `-duplex` | off | Order pages for double-sided printing: a face sheet followed by its back sheet. Pads with blanks to an even number of sheets. |
| `-weld n` | 1 | Number of consecutive pages that stay together (a face group and its back group). Must divide the number of columns. |
| `-flip` | off | Back sheet is mirrored along the columns (turn along the long edge). |
| `-reverse` | off | Back sheet order is reversed. |
| `-turn deg` | 0 | Extra rotation of pages (e.g. `180`). In `booklet` only the face sheets are rotated, so back sheets end up turned against them; in the other commands every page is rotated, like `-angle`. |

### Debug

| Flag | Meaning |
|---|---|
| `-verbosity n` | `1` prints elapsed time, `2` adds pages read/drawn, `3` adds file close. |
| `-cpu file`, `-mem file` | Write Go CPU / heap profiles. |

## Examples

### Grid of pages

```
impose -f in.pdf -grid 2x1 -width 450 -height 320 > out.pdf
```
4 pages on 2 sheets: 1 2 / 3 4.

### Choose pages

```
impose -f in.pdf -pages 1,3     -grid 2x1 -width 450 -height 320 > out.pdf   # Sheet 1: 1 3
impose -f in.pdf -pages 2-4     -grid 2x1 -width 450 -height 320 > out.pdf   # Sheet 1: 2 3, Sheet 2: 4
```

### Size the sheet to the content

```
impose -f in.pdf -pages 1-2 -grid 2x1 -autopage > out.pdf                   # 451 x 351 pt sheet
impose -f in.pdf -pages 1-2 -grid 2x1 -autopage -autopadding 5 > out.pdf    # more room around
impose -f in.pdf -pages 1-2 -grid 2x1 -autopage -center > out.pdf           # marks visible on all four sides
```

### Margins and centering

```
impose -f in.pdf -pages 1-2 -grid 2x1 -left 5 -top 5 -right 5 -bottom 5 -width 450 -height 320 > out.pdf
impose -f in.pdf -pages 1-2 -grid 2x1 -center -width 450 -height 320 > out.pdf
```

### Cropmarks, bleed

```
# cropmarks on all sheets (default); keep room for them with margins
impose -f in.pdf -grid 2x1 -left 12 -top 12 -width 450 -height 320 > out.pdf

# 3 mm bleed in the source, marks 4 mm away from the trim, 6 mm long
impose -f in.pdf -grid 2x1 -bleed 3 -offset 4 -marksize 6 -left 12 -top 12 -width 450 -height 320 > out.pdf

# marks only on the first sheet
impose -f in.pdf -grid 2x1 -cropmarkPages 1 -left 12 -top 12 -width 450 -height 320 > out.pdf

# no marks at all
impose -f in.pdf -grid 2x1 -nocropmark -width 450 -height 320 > out.pdf

# draw the page rectangles (debugging a layout)
impose -f in.pdf -grid 2x1 -outline -nocropmark -width 450 -height 320 > out.pdf
```

### Rotate pages

```
impose -f in.pdf -pages 1-2 -grid 2x1 -angle 90   -width 650 -height 450 > out.pdf
impose -f in.pdf -pages 1-2 -grid 2x1 -angle -90  -autopage > out.pdf
impose -f in.pdf -pages 1-2 -grid 2x1 -angle 180  -width 450 -height 320 > out.pdf
```

### Clone the grid

```
# the 2x1 grid is repeated 2 times along x with a 5 mm gap
impose -f in.pdf -pages 1-2 -grid 1x1 -clone 2x1 -clonepadx 5 -autopage -nocropmark > out.pdf
```

### Repeat pages with `-flow`

`-flow` lists page numbers (from 1) that may repeat. It is applied to every block of `max(flow)` pages.

```
# 2 pages, 3x2 grid: row 1 = page 1 three times, row 2 = page 2 three times
impose -f in2.pdf -grid 3x2 -flow 1,1,1,2,2,2 -nocropmark -width 650 -height 650 > out.pdf

# 4 pages: sheet 1 = 1 1 1 / 2 2 2, sheet 2 = 3 3 3 / 4 4 4
impose -f in.pdf -grid 3x2 -flow 1,1,1,2,2,2 -nocropmark -width 650 -height 650 > out.pdf

# reorder only: 1 3 5 2 4 6, then 7 9 11 8 10 12, ...
impose -f in12.pdf -grid 3x2 -flow 1,3,5,2,4,6 > out.pdf
```
Missing pages of the last block are left empty. `0` and non-numbers are rejected.

### Duplex

Each face sheet is followed by its back sheet.

```
impose -f in8.pdf -duplex -grid 2x1 -nocropmark -width 450 -height 320 > out.pdf
# Sheet 1: 1 3 | Sheet 2: 2 4 | Sheet 3: 5 7 | Sheet 4: 6 8

impose -f in8.pdf -duplex -flip -grid 2x1 -nocropmark -width 450 -height 320 > out.pdf
# Sheet 1: 1 3 | Sheet 2: 4 2 | Sheet 3: 5 7 | Sheet 4: 8 6

impose -f in8.pdf -duplex -weld 2 -grid 4x1 -nocropmark -width 900 -height 320 > out.pdf
# Sheet 1: 1 2 5 6 | Sheet 2: 3 4 7 8

# rotate every page by 180 degrees on top of -angle
impose -f in8.pdf -duplex -turn 180 -grid 2x1 -nocropmark -width 450 -height 320 > out.pdf
```

### `repeat`

Every page is cloned `cols x rows` times on its own sheet.

```
impose repeat 3x1 -f in.pdf -pages 1-2 -nocropmark -width 650 -height 320 > out.pdf
# Sheet 1: 1 1 1 | Sheet 2: 2 2 2

# 2x2 clones with 5 mm gaps, sheet sized automatically, with cropmarks
impose repeat 2x2 -f in.pdf -pages 1-2 -clonepadx 5 -clonepady 5 -autopage > out.pdf
```

### `booklet`

Imposes a folded booklet, default grid `2x1`. The page count (after `-pages`) must be a multiple of 4.

```
impose booklet -f in8.pdf -nocropmark -width 450 -height 320 > out.pdf
# Sheet 1: 8 1 | Sheet 2: 2 7 | Sheet 3: 6 3 | Sheet 4: 4 5

impose booklet -f in8.pdf -grid 4x1 -nocropmark -width 900 -height 320 > out.pdf
# Sheet 1: 8 1 6 3 | Sheet 2: 2 7 4 5

impose booklet -f in8.pdf -autopage > out.pdf
```

`-creep mm` shifts pages sideways progressively, to compensate the thickness of the folded paper. It is limited to the
value of `-bleed`, so use both:

```
impose booklet -f in8.pdf -creep 0.4 -bleed 2 -width 450 -height 320 > out.pdf
```

Booklet accepts the common flags (`-flip`, `-reverse`, `-turn`, margins, cropmarks, ...).

### Units

```
# 17.7 x 12.6 inch sheet; the default marks (2 mm offset, 5 mm size) stay in mm
impose -f in.pdf -pages 1-2 -grid 2x1 -unit in -width 17.7 -height 12.6 > out.pdf

# margins in cm
impose -f in.pdf -pages 1-2 -grid 2x1 -unit cm -left 1 -top 1 -width 45 -height 32 > out.pdf
```

### Profiling and logging

```
impose -f in.pdf -grid 2x1 -verbosity 2 -width 450 -height 320 > out.pdf
impose -f in.pdf -grid 2x1 -cpu cpu.prof -mem mem.prof -width 450 -height 320 > out.pdf
```

## Desktop window

`gui/` is a window for the same engine (Windows and Linux): a form with every setting, a live preview of the sheets
and a button that writes the PDF.

```
make gui                  # bin/impose-gui, needs cgo (see the Makefile)
make gui-windows          # bin/impose-gui.exe, cross build with mingw-w64
bin/impose-gui [file.pdf]
```

- **Opening a PDF** can be done five ways: **Find...** (Ctrl+F), the **Open PDF...** button, the **Recent files**
  dropdown (the last 10, kept between runs), pasting or typing a path in the entry below them and pressing Enter (quotes,
  `file://` addresses and `~` are understood), or dropping the file on the window. A file given in the command line is
  opened too. The page count and page size are shown once it is read; if a file can not be opened, the one that was
  open stays.
- **Find...** searches for a PDF by name. Type part of the name (several words must all be in it, capitals do not
  matter) and the PDFs of the folder and the folders under it are listed as you type, newest first, with their folder,
  size and date. Choose one and press **Open**, or press Enter to open the first. *In* is the folder to look in (type
  it, pick it with the folder button, or use *Go to...* for Home, Documents, Downloads, Desktop and the folder of the
  last file); *Also match folder names* lets `contracts 2024` find `Contracts/2024/final.pdf`. Hidden folders (starting
  with a dot), `node_modules`, folders that can not be read and links to folders are skipped, and at most 500 results
  are shown, so a short name in a big folder asks you to type more. The search runs in the background and restarts or
  stops when you type, change folder or close the dialog.
- Choose **Grid**, **Repeat** or **Booklet**; only the settings that apply to the mode are shown. **Unit** changes the
  unit of all the lengths without changing their size.
- The preview follows every change after a short pause. It draws one sheet at a time: the sheet, margins (red line),
  page boxes with their page number and angle, the top of each page (orange edge) and the cropmarks. Duplex and booklet
  sheets are marked *face* or *back*. Errors are shown where the preview would be, like `2 columns do not fit`.
  When the grid is empty it tells how many pages fit and offers to use it.
- **Same as:** shows the command line that gives the same PDF, **Copy command** puts it in the clipboard.
- **Generate PDF...** writes the file in the background and can be cancelled. A cancelled or failed run leaves no file.
- Settings are kept between runs, and named **presets** can be saved (disk icon), deleted (bin icon) or the form reset.
  They live in `gui.json` in the user config folder (`~/.config/impose` on Linux, `%AppData%\impose` on Windows).

The preview is drawn from the layout the program computes, it is not a picture of the PDF; it shows where every page
goes, in which order and with which rotation.

## Errors

| Message | Cause |
|---|---|
| `pdf file required` | `-f` missing. |
| `N columns / rows do not fit` | Grid (with clones and pads) is larger than the sheet minus margins. |
| `N is not divisible with 4` | `booklet` page count is not a multiple of 4. |
| `flow: ...` | `-flow` has a value below 1 or a non-number. |
| `suggested grid CxR` | `-grid` omitted (non-booklet): the program prints how many pages fit on the sheet (margins, clones and pads included) and exits. Run again with that `-grid`. |
| `not even one page fits on the sheet` | `-grid` omitted and the page is larger than the sheet. |
| `repeat needs a grid, ex: repeat 3x2` | `repeat` without its `CxR` argument. |
| `unknown unit ...` | `-unit` is not `mm`, `cm`, `in` or `pt`. |

## Known limitations

- Unlicensed unipdf banner on stdout (see above).
