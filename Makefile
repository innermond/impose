# impose: command line and desktop window.
#
# The window (gui/) is built with Fyne, it needs cgo: a C compiler, and on Linux the OpenGL and X11
# development packages (Debian: libgl1-mesa-dev xorg-dev). A Windows build from Linux needs mingw-w64.

BIN := bin

.PHONY: all cli gui gui-windows test clean

all: cli gui

cli:
	go build -o $(BIN)/impose ./cmd/cli

# if the linker can not find a library, pass its place: make gui CGO_LDFLAGS=-L/some/dir
gui:
	cd gui && CGO_ENABLED=1 go build -o ../$(BIN)/impose-gui .

gui-windows:
	cd gui && CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
		go build -ldflags "-H=windowsgui -s -w" -o ../$(BIN)/impose-gui.exe .

test:
	go test ./...
	cd gui && CGO_ENABLED=1 go test ./...

clean:
	rm -rf $(BIN)
