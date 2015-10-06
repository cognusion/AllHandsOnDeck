// +build go1.4
// +build windows

// Windows-specific stuff goes here, so we can accomodate the broken Windows.
package main

import (
	"runtime"
)

func saneMaxLimit(sessionCount int) int {
	return runtime.GOMAXPROCS(0)
}
