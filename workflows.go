// +build go1.4

package main

import (
	"fmt"
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
func (w *Workflow) Exec(com *Command) WorkflowReturn {

	var wr WorkflowReturn
	wr.Name = w.Name
	wr.HostObj = com.Host
	wr.Completed = false

	debugOut.Printf("Executing workflow %s\n", w.Name)

	// Per-wf override for sudo
	if w.Sudo == true {
		com.Sudo = true
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
		} else if strings.HasPrefix(c, "SET ") {
			// SET %varname% "some string"
			err := w.handleSet(c)
			if err != nil {
				log.Printf("Error during SET: %s\n", err)
				return wr
			}
		} else if strings.HasPrefix(c, "FOR ") {
			// FOR list ACTION
			crs, err := w.handleFor(c, com)
			if len(crs) > 0 {
				wr.CommandReturns = append(wr.CommandReturns, crs...)
			}
			if err != nil {
				log.Printf("Error during FOR: %s\n", err)
				return wr
			}
		} else {
			// Regular command
			com.Cmd = c
			res := com.Exec()
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

func (w *Workflow) handleFor(c string, com *Command) ([]CommandReturn, error) {

	var crs []CommandReturn

	cparts := strings.Split(c, " ")
	if len(cparts) < 3 {
		// Hmmm, malformated FOR
		return crs, fmt.Errorf("'FOR list ACTION' statement incomplete: '%s'\n", c)
	}

	var list []string

	// Set up our list
	if cparts[1] == "needs-restarting" {
		// Do that voodoo that you do, for special command "needs-restarting"
		com.Cmd = cparts[1]
		listRes := com.Exec()
		crs = append(crs, listRes)
		if listRes.Error != nil {
			// We have a valid error, and must break, as we'll have
			//  an invalid list to operate on
			return crs, fmt.Errorf("needs-restarting on host %s failed: %s\n", com.Host.Name, listRes.Error)
		}
		list = needsRestartingMangler(listRes.StdoutStrings(true), makeList([]string{globalVars["dontrestart-processes"]}))
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
		serviceList(action, list, serviceResults, com)

		for li := 0; li < len(list); li++ {
			select {
			case res := <-serviceResults:
				crs = append(crs, res)
			}
		}
	}

	return crs, nil

}

func (w *Workflow) handleSet(c string) error {

	var err error
	cparts := strings.Split(c, " ")

	if len(cparts) < 3 {
		// Hmmm, malformated SET
		return fmt.Errorf("'SET %varname% \"value\"' statement incomplete: '%s'\n", c)
	}

	vname := strings.Trim(cparts[1], "%")              // nuke the lead/trail percents from the varname
	vvalue := strings.Join(cparts[2:len(cparts)], " ") // concatenate any end parts
	vvalue = strings.Trim(vvalue, "\"")                // nuke lead/trail dub-quotes
	vvalue = strings.Trim(vvalue, "'")                 // nuke lead/trail sing-quotes

	if strings.Contains(vvalue, "S3(") {
		// We need a tokened S3 URL
		vvalue, err = w.handleS3(vvalue, globalVars["awsaccess_key"], globalVars["awsaccess_secretkey"])
	} else if strings.Contains(vvalue, "RAND(") {
		// We need a random string
		vvalue, err = w.handleRand(vvalue)
	}

	if err != nil {
		return err
	}

	w.vars[vname] = vvalue
	return nil
}

func (w *Workflow) handleS3(vvalue, accessKey, secretKey string) (string, error) {

	// Confirm we actually have the bits set
	if _, ok := globalVars["awsaccess_key"]; ok == false {
		return "", fmt.Errorf("No AWS access key set, but S3() called\n")
	} else if _, ok := globalVars["awsaccess_secretkey"]; ok == false {
		return "", fmt.Errorf("No AWS secret key set, but S3() called\n")
	}

	re := regexp.MustCompile(`^(.*)S3\((.*)\)(.*)$`)
	rparts := re.FindStringSubmatch(vvalue)
	if rparts == nil {
		return "", fmt.Errorf("Error processing S3(s): '%s'\n", vvalue)
	}

	s3u := s3UrlToParts(rparts[2])
	url := generateS3Url(s3u.Bucket, s3u.Path,
		accessKey, secretKey, "", 60)

	return rparts[1] + url + rparts[3], nil
}

func (w *Workflow) handleRand(vvalue string) (string, error) {

	re := regexp.MustCompile(`^(.*)RAND\(([0-9]+)\)(.*)$`)
	rparts := re.FindStringSubmatch(vvalue)
	if rparts == nil {
		return "", fmt.Errorf("Error processing RAND(n): '%s'\n", vvalue)
	}

	n, err := strconv.Atoi(rparts[2])
	if err != nil {
		return "", fmt.Errorf("Problem using '%s' as a number\n", rparts[2])
	}
	return rparts[1] + randString(n) + rparts[3], nil
}
