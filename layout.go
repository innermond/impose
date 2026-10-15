package impose

// Placement is where a page lands on a sheet, in points, from the top left corner of the sheet.
type Placement struct {
	// Page is the number of the page of the source pdf
	Page int
	X, Y float64
	W, H float64
	// Angle is the rotation of the page, in degrees
	Angle float64
}

// Line is a segment, in points, from the top left corner of the sheet.
type Line struct {
	X1, Y1, X2, Y2 float64
}

// Sheet is a sheet of the imposition.
type Sheet struct {
	Placements []Placement
	// Marks are the cropmarks drawn on this sheet, none if the sheet has no cropmarks.
	Marks []Line
}

// Layout describes every sheet of an imposition without drawing any pdf.
type Layout struct {
	// Width and Height of every sheet.
	Width, Height float64
	// Margins in use, after centering.
	Top, Right, Bottom, Left float64
	// Col and Row are the grid, CloneX and CloneY the clones of the grid.
	Col, Row, CloneX, CloneY int
	Sheets                   []Sheet
}
