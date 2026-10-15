// Command gui is a desktop window for impose: a form with all the settings, a preview of the
// sheets and a button that writes the imposed pdf.
package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.NewWithID("io.github.innermond.impose")
	w := a.NewWindow("impose")
	file, _ := storePath()
	u := newUI(a, w, file)
	w.SetOnClosed(u.close)
	w.Resize(fyne.NewSize(1200, 800))
	// a pdf given in the command line is opened
	if len(os.Args) > 1 {
		u.openFile(os.Args[1])
	}
	w.ShowAndRun()
}
