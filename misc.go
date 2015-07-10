package main

import (
	"strings"
)

func needsRestartingMangler(plist []string) []string {

	// We make a map to get free dedup prior to listing
	initMap := make(map[string]bool)
	var initList []string
	
	for _,p := range plist {
		cmds := strings.SplitN(p," : ",2)
		if len(cmds) > 1 {
			cmds = strings.SplitN(cmds[1]," ",2)
			cmd := cmds[0]
			
			// Split up pathnames
			if strings.HasPrefix(cmd, "/") {
				cmds = strings.Split(cmd,"/")
				cmd = cmds[len(cmds)-1]
			}
			
			// Clean up control names
			if strings.HasSuffix(cmd, ":") {
				cmds = strings.Split(cmd,":")
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
	for p,_ := range initMap {
		initList = append(initList, p)
	}
	 
	return initList
}