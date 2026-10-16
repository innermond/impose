//go:build cgo

package main

// The MuPDF of go-fitz for Windows is built for the UCRT runtime, which names _setjmpex
// __intrinsic_setjmpex; mingw-w64 builds for msvcrt, which has only _setjmpex.
// setjmp_windows_amd64.S gives the name, cgo is needed to build it with the C compiler.

import "C"
