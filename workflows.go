package main

import (
	"golang.org/x/crypto/ssh"
	"strings"
)

type WorkflowReturn struct {
	Name           string
	HostObj        Host
	Completed      bool
	CommandReturns []CommandReturn
}

type Workflow struct {
	Name          string
	Filter        string
	Sudo          bool
	Commands      []string
	CommandBreaks []bool
}

func (w *Workflow) Exec(host Host, config *ssh.ClientConfig, sudo bool) WorkflowReturn {

	var wr WorkflowReturn
	wr.Name = w.Name
	wr.HostObj = host
	wr.Completed = false

	// Per-wf override for sudo
	if w.Sudo == true {
		sudo = true
	}

	for i, c := range w.Commands {
		// Handle workflow special commands
		if strings.HasPrefix(c,"FOR ") {
			cparts := strings.Split(c, " ")
			if len(cparts) != 3 {
				// Hmmm, malformated FOR
				return wr //?
			}
			
			listRes := executeCommand(cparts[1], host, config, sudo)
			wr.CommandReturns = append(wr.CommandReturns, listRes)
			if listRes.Error != nil && (len(w.CommandBreaks) == 0 || w.CommandBreaks[i] == true) {
				// We have a valid error, and either we're not using CommandBreaks (assume breaks)
				//	or we are using CommandBreaks, and they're true
				return wr
			} else if cparts[2] == "RESTART" && cparts[1] == "needs-restarting" {
				// Restart requested.
				
				plist := needsRestartingMangler(listRes.StdoutStrings())
				restartResults := make(chan CommandReturn, 10)
				for _,p := range plist {
					restartCommand := "service " + p + " restart"
					
					go func(host Host) {
						restartResults <- executeCommand(restartCommand, host, config, sudo)
					}(host)
					
				}
				
				for pi := 0; pi < len(plist); pi++ {
					select {
					case res := <-restartResults:
						wr.CommandReturns = append(wr.CommandReturns, res)
					}
				}
			}
			
		} else {
			// Regular command
			res := executeCommand(c, host, config, sudo)
			wr.CommandReturns = append(wr.CommandReturns, res)
			if res.Error != nil && (len(w.CommandBreaks) == 0 || w.CommandBreaks[i] == true) {
				// We have a valid error, and either we're not using CommandBreaks (assume breaks)
				//	or we are using CommandBreaks, and they're true
				return wr
			}
		}
	}
	// POST: No errors

	wr.Completed = true
	return wr
}
