// +build go1.4
// +build !windows,!plan9

// UNIX-specific stuff goes here, so we do not break the Windows.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func getOpenFiles() []string {
	out, err := exec.Command("/bin/bash", "-c", fmt.Sprintf("lsof -np %v", os.Getpid())).Output()
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(out), "\n")
	return lines
}

func saneMaxLimit(sessionCount int) int {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Fatal("Error Getting Rlimit: %v\n", err)
	}

	of := len(getOpenFiles()) - 1
	oflimit := int(rLimit.Cur)
	avail := oflimit - of

	if sessionCount < 1 {
		// Sanity
		sessionCount = 1
	}

	Debug.Printf("Open files: %d of %d (%d avail). Session count: %d\n", of, oflimit, avail, sessionCount)

	return avail / (sessionCount * 2)
}
