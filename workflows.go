// +build go1.4

package main

import (
	"golang.org/x/crypto/ssh"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// WorkflowReturn is a structure returned after executing a workflow
type WorkflowReturn struct {
	Name           string
	HostObj        Host
	Completed      bool
	CommandReturns []CommandReturn
}

// Workflow is a structure to capture properties of an individual workflow
type Workflow struct {
	Filter        string
	Name          string
	Sudo          bool
	MinTimeout    int
	Commands      []string
	CommandBreaks []bool
	vars          map[string]string
}

func (w *Workflow) varParse(s string) string {

	// First check the global list
	for k, v := range globalVars {
		nk := "%" + k + "%"
		if strings.Contains(s, nk) {
			s = strings.Replace(s, nk, v, -1)
		}
	}

	// Then the workflow list
	for k, v := range w.vars {
		nk := "%" + k + "%"
		if strings.Contains(s, nk) {
			s = strings.Replace(s, nk, v, -1)
		}
	}

	return s
}

// Exec executes a workflow against the supplied Host
func (w *Workflow) Exec(host Host, config *ssh.ClientConfig, sudo bool) WorkflowReturn {

	var wr WorkflowReturn
	wr.Name = w.Name
	wr.HostObj = host
	wr.Completed = false

	debugOut.Printf("Executing workflow %s\n", w.Name)

	// Per-wf override for sudo
	if w.Sudo == true {
		sudo = true
	}

	for i, c := range w.Commands {

		// Check the command for variables
		if strings.HasPrefix(c, "SET ") == false {
			c = w.varParse(c)
		}

		// Handle workflow special commands
		if strings.HasPrefix(c, "%%") {
			// %%anotherworkflowname
			log.Printf("Chaining workflows currently unsupported!\n")
			return wr

			/*
				flow := strings.TrimPrefix(c, "%%")
				flowIndex := conf.WorkflowIndex(flow)
				if flowIndex < 0 {
					log.Printf("Chained workflow '%s' does not exist in specified configs!\n", flow)
					return wr
				}
			*/

		} else if strings.HasPrefix(c, "SET ") {
			// SET %varname% "some string"

			cparts := strings.Split(c, " ")
			if len(cparts) < 3 {
				// Hmmm, malformated SET
				log.Printf("'SET %varname% \"value\"' statement incomplete: '%s'\n", c)
				return wr //?
			}

			vname := strings.Trim(cparts[1], "%")              // nuke the lead/trail percents from the varname
			vvalue := strings.Join(cparts[2:len(cparts)], " ") // concatenate any end parts
			vvalue = strings.Trim(vvalue, "\"")                // nuke lead/trail dub-quotes
			vvalue = strings.Trim(vvalue, "'")                 // nuke lead/trail sing-quotes

			if strings.Contains(vvalue, "S3(") {
				// We need a tokened S3 URL

				// Confirm we actually have the bits set
				if _, ok := globalVars["awsaccess_key"]; ok == false {
					log.Printf("No AWS access key set, but S3() called\n")
					return wr
				} else if _, ok := globalVars["awsaccess_secretkey"]; ok == false {
					log.Printf("No AWS secret key set, but S3() called\n")
					return wr
				}

				re := regexp.MustCompile(`^(.*)S3\((.*)\)(.*)$`)
				rparts := re.FindStringSubmatch(vvalue)
				if rparts == nil {
					log.Printf("Error processing S3(s): '%s'\n", vvalue)
					return wr
				}

				bucket, filePath, _ := s3UrlToParts(rparts[2])
				url := generateS3Url(bucket, filePath,
					globalVars["awsaccess_key"], globalVars["awsaccess_secretkey"],
					"", 60)

				vvalue = rparts[1] + url + rparts[3]

			} else if strings.Contains(vvalue, "RAND(") {
				// We need a random string
				re := regexp.MustCompile(`^(.*)RAND\(([0-9]+)\)(.*)$`)
				rparts := re.FindStringSubmatch(vvalue)
				if rparts == nil {
					log.Printf("Error processing RAND(n): '%s'\n", vvalue)
					return wr
				}

				n, err := strconv.Atoi(rparts[2])
				if err != nil {
					log.Printf("Problem using '%s' as a number\n", rparts[2])
					return wr
				}
				vvalue = rparts[1] + randString(n) + rparts[3]
			}

			w.vars[vname] = vvalue

		} else if strings.HasPrefix(c, "FOR ") {
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
				if listRes.Error != nil {
					// We have a valid error, and must break, as we'll have
					//  an invalid list to operate on
					log.Printf("needs-restarting on host %s failed: %s\n", host.Name, listRes.Error)
					return wr
				}
				list = needsRestartingMangler(listRes.StdoutStrings(true),makeList([]string{globalVars["dontrestart-processes"]}))
			} else {
				// Treat the middle of cparts as actual list items
				list = makeList(cparts[1 : len(cparts)-1])
			}

			// Handle our ACTIONs
			action := strings.ToLower(cparts[len(cparts)-1])
			if action == "restart" ||
				action == "start" ||
				action == "stop" ||
				action == "status" {
				// Service operation requested.

				serviceResults := make(chan CommandReturn, 10)
				serviceList(action, list, serviceResults, host, config, sudo)

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
