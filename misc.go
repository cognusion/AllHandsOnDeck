// +build go1.4

package main

import (
	"crypto/rand"
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

type Misc struct {
	Name  string
	Value string
}

func miscToMap(miscs []Misc) map[string]string {
	mss := make(map[string]string)
	for _, m := range miscs {
		mss[m.Name] = m.Value
	}
	return mss
}

func loadConfigs(srcDir string) Config {
	var conf Config
	for _, f := range readDirectory(srcDir) {
		conf = loadFile(f, conf)
	}
	return conf
}

func readDirectory(srcDir string) []string {
	// We can skip this error, since our pattern is fixed and known-good.
	files, _ := filepath.Glob(srcDir + "*.json")
	return files
}

func loadFile(filePath string, conf Config) Config {

	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading config file '%s': %s\n", filePath, err)
	}

	// In any language but Go, the below would concerningly
	// overwrite the conf struct, but in Go, it "merges"
	// automagically.
	err = json.Unmarshal(buf, &conf)
	if err != nil {
		log.Fatalf("Error parsing JSON in config file '%s': %s\n", filePath, err)
	}

	return conf
}

func needsRestartingMangler(plist []string) []string {

	// We make a map to get free dedup prior to listing
	initMap := make(map[string]bool)
	var initList []string

	for _, p := range plist {
		cmds := strings.SplitN(p, " : ", 2)
		if len(cmds) > 1 {
			cmds = strings.SplitN(cmds[1], " ", 2)
			cmd := cmds[0]

			// Split up pathnames
			if strings.HasPrefix(cmd, "/") {
				cmds = strings.Split(cmd, "/")
				cmd = cmds[len(cmds)-1]
			}

			// Clean up control names
			cmd = strings.TrimSuffix(cmd, ":")

			if cmd == "mongod" || cmd == "udevd" {
				// Do Not Want
			} else if cmd == "haproxy" {
				initMap["haproxy"] = true
			} else if cmd == "java" && strings.Contains(p, "catalina") {
				initMap["tomcat"] = true
			} else if strings.HasSuffix(cmd, "d") {
				if cmd == "rsyslogd" {
					// Stupid thing
					cmd = "rsyslog"
				}
				initMap[cmd] = true
			}
		}
	}

	// Deduped, so now we make a simple array
	for p, _ := range initMap {
		initList = append(initList, p)
	}

	return initList
}

func makeList(list []string) []string {
	var newList []string
	for _, tc := range list {
		if strings.Contains(tc, ",") {
			// Handle comma hell
			tc = strings.TrimSuffix(tc, ",") // Nuke trailing commas
			nl := strings.Split(tc, ",")     // Split out any comma-sep
			newList = sliceAppend(newList, nl)
		} else {
			newList = append(newList, tc)
		}
	}
	return newList
}

func sliceAppend(slice []string, elements []string) []string {
	n := len(slice)
	total := len(slice) + len(elements)
	if total > cap(slice) {
		// Reallocate. Grow to 1.5 times the new size, so we can still grow.
		newSize := total*3/2 + 1
		newSlice := make([]string, total, newSize)
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[:total]
	copy(slice[n:], elements)
	return slice
}

func randString(size int) string {
	chars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := make([]byte, size)

	rand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = chars[v%byte(len(chars))]
	}
	return string(bytes)
}
