// +build go1.4

package main

import (
	"golang.org/x/crypto/ssh"
	"log"
	"strings"
)

type WorkflowReturn struct {
	Name           string
	HostObj        Host
	Completed      bool
	CommandReturns []CommandReturn
}

type Workflow struct {
	Filter        string
	Name          string
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
		if strings.HasPrefix(c, "FOR ") {
			// FOR list ACTION

			cparts := strings.Split(c, " ")
			if len(cparts) < 3 {
				// Hmmm, malformated FOR
				log.Printf("'FOR list ACTION' statement incomplete: '%s'\n", c)
				return wr //?
			}

			var list []string

			// Set up our list
			if cparts[1] == "needs-restarting" {
				// Do that voodoo that you do, for special command "needs-restarting"
				listRes := executeCommand(cparts[1], host, config, sudo)
				wr.CommandReturns = append(wr.CommandReturns, listRes)
				if listRes.Error != nil && (len(w.CommandBreaks) == 0 || w.CommandBreaks[i] == true) {
					// We have a valid error, and either we're not using CommandBreaks (assume breaks)
					//	or we are using CommandBreaks, and they're true
					return wr
				}
				list = needsRestartingMangler(listRes.StdoutStrings())
			} else {
				// Treat the middle of cparts as actual list items
				var newList []string
				for _, tc := range cparts[1 : len(cparts)-1] {
					if strings.Contains(tc, ",") {
						// Handle comma hell
						tc = strings.TrimSuffix(tc, ",") // Nuke trailing commas
						nl := strings.Split(tc, ",")     // Split out any comma-sep
						newList = sliceAppend(newList, nl)
					} else {
						newList = append(newList, tc)
					}
				}
				list = newList
			}

			// Handle our ACTIONs
			if cparts[len(cparts)-1] == "RESTART" ||
				cparts[len(cparts)-1] == "START" ||
				cparts[len(cparts)-1] == "STOP" ||
				cparts[len(cparts)-1] == "STATUS" {
				// Service operation requested.

				op := strings.ToLower(cparts[len(cparts)-1])

				serviceResults := make(chan CommandReturn, 10)

				serviceList(op, list, serviceResults, host, config, sudo)

				for li := 0; li < len(list); li++ {
					select {
					case res := <-serviceResults:
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
