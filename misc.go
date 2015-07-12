// +build go1.4

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

func loadConfigs(srcDir string) Config {
	var conf Config
	for _, f := range readDirectory(srcDir) {
		conf = loadFile(f, conf)
	}
	return conf
}

func readDirectory(srcDir string) []string {
	files, _ := filepath.Glob(srcDir + "*.json")
	return files
}

func loadFile(filePath string, conf Config) Config {

	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}

	// In any language but Go, the below would concerningly
	// overwrite the conf struct, but in Go, it "merges"
	// automagically.
	err = json.Unmarshal(buf, &conf)
	if err != nil {
		log.Fatal(err)
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
			if strings.HasSuffix(cmd, ":") {
				cmds = strings.Split(cmd, ":")
				cmd = cmds[0]
			}

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
