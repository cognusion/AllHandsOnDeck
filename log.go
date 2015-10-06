package main

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	Debug *log.Logger
	Log   *log.Logger
	Error *log.Logger
)

// Ensure we have sane defaults from the start
func init() {
	Debug = log.New(ioutil.Discard, "", log.Lshortfile)
	Log = log.New(os.Stdout, "", 0)
	Error = log.New(os.Stderr, "", 0)
}

// Set the debug log
func SetDebug(filename string) {
	if filename == "" {
		Debug = log.New(os.Stderr, "[DEBUG]", log.Lshortfile)
	} else if lFile, err := openFile(filename); err == nil {
		Debug = log.New(lFile, "[DEBUG]", log.Lshortfile)
	} else {
		if err != nil {
			log.Fatalf("Error opening log file '%s': %v\n", filename, err)
		}
	}
}

// Set the standard log
func SetLog(filename string) {
	if filename == "" {
		Log = log.New(os.Stdout, "", 0)
	} else if lFile, err := openFile(filename); err == nil {
		Log = log.New(lFile, "", 0)
	} else {
		if err != nil {
			log.Fatalf("Error opening log file '%s': %v\n", filename, err)
		}
	}
}

// Set the error log
func SetError(filename string) {
	if filename == "" {
		Error = log.New(os.Stdout, "", 0)
	} else if lFile, err := openFile(filename); err == nil {
		Error = log.New(lFile, "", 0)
	} else {
		if err != nil {
			log.Fatalf("Error opening log file '%s': %v\n", filename, err)
		}
	}
}

func openFile(filename string) (*os.File, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return file, nil
}
