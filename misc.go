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

// Misc is a simple key/value structure
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

func dumpConfigs(conf Config) string {
	j, _ := json.MarshalIndent(conf, "", "\t")
	return string(j)
}

func loadConfigs(srcDir string) Config {
	var conf Config
	debugOut.Printf("Looking for configs in '%s'\n", srcDir)
	for _, f := range readDirectory(srcDir) {
		debugOut.Printf("\tReading config '%s'\n", f)
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

	var newConf Config
	err = json.Unmarshal(buf, &newConf)
	if err != nil {
		log.Fatalf("Error parsing JSON in config file '%s': %s\n", filePath, err)
	}

	conf.Merge(newConf)
	return conf
}

func needsRestartingMangler(plist []string, drList []string) []string {

	// We make a map to get free dedup prior to listing
	initMap := make(map[string]bool)
	dontrestart := make(map[string]bool)
	var initList []string

	for _, v := range drList {
		dontrestart[strings.TrimSpace(v)] = true
	}

	for _, p := range plist {
		p := strings.TrimSpace(p)
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

			// See if we need to skip this
			if _, ok := dontrestart[cmd]; ok {
				// Yup, we need to skip this
				continue
			}

			if cmd == "sshd" && (strings.Contains(p, "[priv]") || strings.Contains(p, "@pts")) {
				// skip sshd if it's just connections
			} else if cmd == "sendmail" {
				initMap["sendmail"] = true
			} else if cmd == "haproxy" {
				initMap["haproxy"] = true
			} else if cmd == "ns-slapd" {
				initMap["dirsrv"] = true
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

// given a list of strings, explode any comma-lists embedded therein
func makeList(list []string) []string {
	var newList []string
	for _, tc := range list {
		if strings.Contains(tc, ",") {
			// Handle comma hell
			tc = strings.TrimSuffix(tc, ",") // Nuke trailing commas
			nl := strings.Split(tc, ",")     // Split out any comma-sep
			newList = append(newList, nl...)
		} else {
			newList = append(newList, tc)
		}
	}
	return newList
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
