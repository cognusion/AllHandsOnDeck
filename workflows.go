package main

import (
	"strings"
	"golang.org/x/crypto/ssh"
)

type WorkflowReturn struct {
	Name           string
	HostObj        Host
	Completed      bool
	CommandReturns []CommandReturn
}

type Workflow struct {
	Name string
	Commands []string
	ComBreaks []bool
}

func (w *Workflow) Exec(host Host, config *ssh.ClientConfig, pty bool) WorkflowReturn {

	var wr WorkflowReturn
	wr.Name = w.Name
	wr.HostObj = host
	wr.Completed = false
	
	for i,c := range w.Commands {
		// TODO: Handle "WF" special commands
		res := executeCommand(c, host, config, pty)
		wr.CommandReturns = append(wr.CommandReturns,res)
		if res.Error != nil && w.ComBreaks[i] == true {
			return wr
		}
	}
	// POST: No errors
	
	wr.Completed = true
	return wr
}

func plistToInits(plist []string) []string {

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
			
			
			if cmd == "java" && strings.Contains(p, "catalina") {
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