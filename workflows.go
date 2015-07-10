package main

import (
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

func (w *Workflow) Exec(host Host, config *ssh.ClientConfig, sudo bool) WorkflowReturn {

	var wr WorkflowReturn
	wr.Name = w.Name
	wr.HostObj = host
	wr.Completed = false
	
	for i,c := range w.Commands {
		// TODO: Handle "WF" special commands
		res := executeCommand(c, host, config, sudo)
		wr.CommandReturns = append(wr.CommandReturns,res)
		if res.Error != nil && (len(w.ComBreaks) == 0 || w.ComBreaks[i] == true) {
			// We have a valid error, and either we're not using ComBreaks (assume breaks)
			//	or we are using ComBreaks, and they're true
			return wr
		}
	}
	// POST: No errors
	
	wr.Completed = true
	return wr
}
