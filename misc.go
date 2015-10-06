// +build go1.4
// +build !plan9

package main

import (
	"crypto/rand"
	"path/filepath"
	"strings"
)

var GlobalVars map[string]string

// Misc is a simple key/value structure
type Misc struct {
	Name  string
	Value string
}

// Given an array of Miscs, give us a map[string]string
func miscToMap(miscs []Misc) map[string]string {
	mss := make(map[string]string)
	for _, m := range miscs {
		mss[m.Name] = m.Value
	}
	return mss
}

func readDirectory(srcDir, pattern string) []string {
	// We can skip this error, since our pattern is fixed and known-good.
	files, _ := filepath.Glob(srcDir + pattern)
	return files
}

// The yum tool "needs-restarting" is a very underutilized beast, that
// identifies running processes that predate the latest version of required
// libraries, packages, etc.
//
// This helper function takes that output, and a list of processes to never
// restart, and creates a list of likely init scripts to operate on.
func needsRestartingMangler(plist, drList []string) (initList []string) {

	// We make a map to get free dedup prior to listing
	initMap := make(map[string]bool)
	dontrestart := make(map[string]bool)

	for _, v := range drList {
		dontrestart[strings.TrimSpace(v)] = true
	}

	for _, p := range plist {
		p := strings.TrimSpace(p)
		cmds := strings.SplitN(p, " : ", 2)
		if len(cmds) > 1 {
			cmds = strings.SplitN(cmds[1], " ", 2)
			cmd := cmds[0]

			if strings.HasPrefix(cmd, "/") {
				// Split up pathnames
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

			// Note that if a process doesn't end with "d" (daemon)
			// we need to handle it directly.
			// TODO: Abstract into a list?
			if cmd == "sshd" && (strings.Contains(p, "[priv]") || strings.Contains(p, "@pts")) {
				// skip sshd if it's just connections
			} else if cmd == "sendmail" {
				initMap["sendmail"] = true
			} else if cmd == "haproxy" {
				initMap["haproxy"] = true
			} else if cmd == "ns-slapd" {
				// 389ds' init script is "dirsrv"
				initMap["dirsrv"] = true
			} else if cmd == "java" && strings.Contains(p, "catalina") {
				// There can be javas other than tomcat, so we filter
				initMap["tomcat"] = true
			} else if cmd == "nagios" {
				initMap["nagios"] = true
			} else if strings.HasSuffix(cmd, "d") {
				// Anything ending in "d" (daemon) is fair game
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

	return
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

// Return a randomish string of the specified size
func randString(size int) string {
	chars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := make([]byte, size)

	rand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = chars[v%byte(len(chars))]
	}
	return string(bytes)
}
