// +build go1.4

package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	MustChain     bool
	Commands      []string
	CommandBreaks []bool
	VarsRequired  []string
	vars          map[string]string
}

// Merge another uninitialized Workflow into this one
func (w *Workflow) Merge(other *Workflow) {

	// Set filter if it's empty, otherwise AND it if it isn't there already
	if w.Filter == "" && other.Filter != "" {
		w.Filter = other.Filter
	} else if other.Filter != "" {
		if !strings.Contains(w.Filter, other.Filter) {
			w.Filter = fmt.Sprintf("%s && %s", w.Filter, other.Filter)
		}
	}

	// Sudo if we need it
	if other.Sudo {
		w.Sudo = true
	}

	// Ensure the max min
	if other.MinTimeout > w.MinTimeout {
		w.MinTimeout = other.MinTimeout
	}

	// Append all the arrays
	w.Commands = append(w.Commands, other.Commands...)
	w.CommandBreaks = append(w.CommandBreaks, other.CommandBreaks...)
	w.VarsRequired = append(w.VarsRequired, other.VarsRequired...)

}

func (w *Workflow) varParse(s string) string {

	// First check the global list
	for k, v := range GlobalVars {
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

func (w *Workflow) Init() {
	w.vars = make(map[string]string)

	// Prime the SET pump
	for i, c := range w.Commands {
		if strings.HasPrefix(c, "SET ") {
			// SET %varname% "some string"
			err := w.handleSet(c)
			if err != nil {
				log.Printf("Error during SET: %s\n", err)
			}
		} else {
			// Expand the vars, so we don't have to do it
			// later, a billion times
			w.Commands[i] = w.varParse(c)
		}
	}

}

// Exec executes a workflow against the supplied Host
func (w *Workflow) Exec(com Command) (wr WorkflowReturn) {

	wr = WorkflowReturn{
		Name:      w.Name,
		HostObj:   com.Host,
		Completed: false,
	}

	Debug.Printf("Executing workflow %s\n", w.Name)

	// Per-wf override for sudo
	if w.Sudo == true {
		com.Sudo = true
	}

	for i, c := range w.Commands {

		if strings.HasPrefix(c, "#") {
			// Comment
			continue
		}

		if strings.HasPrefix(c, "SET ") {
			// SET %varname% "some string"
			// Handled by Init()
			continue
		}

		// Handle workflow special commands
		if strings.HasPrefix(c, "QUIET ") {
			// Set quiet, and mangle the command
			c = strings.TrimPrefix(c, "QUIET ")
			Debug.Printf("Command quieted: '%s'\n", c)
			com.Quiet = true
		} else {
			// Make sure we're not quiet
			com.Quiet = false
		}

		if strings.HasPrefix(c, "%%") {
			// %%anotherworkflowname
			log.Printf("Chaining workflows currently unsupported!\n")
			return

		} else if strings.HasPrefix(c, "FOR ") {
			// FOR list ACTION
			crs, err := w.handleFor(c, com)
			if len(crs) > 0 {
				wr.CommandReturns = append(wr.CommandReturns, crs...)
			}
			if err != nil {
				log.Printf("Error during FOR: %s\n", err)
				return
			}
		} else if strings.HasPrefix(c, "SLEEP ") {
			// SLEEP DURATION
			c = strings.TrimPrefix(c, "SLEEP ")
			err := w.handleSleep(c)
			if err != nil {
				log.Printf("Error during SLEEP: %s\n", err)
				// non-fatal
			}
		} else {
			// Regular command
			com.Cmd = c
			res := com.Exec()
			wr.CommandReturns = append(wr.CommandReturns, res)
			if res.Error != nil && (len(w.CommandBreaks) == 0 || w.CommandBreaks[i] == true) {
				// We have a valid error, and either we're not using CommandBreaks (assume breaks)
				//	or we are using CommandBreaks, and they're true
				return
			}
		}
	}
	// POST: No errors

	wr.Completed = true
	return
}

func (w *Workflow) handleFor(c string, com Command) ([]CommandReturn, error) {

	var crs []CommandReturn

	cparts := strings.Fields(c)
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
		list = needsRestartingMangler(listRes.StdoutStrings(true), makeList([]string{GlobalVars["dontrestart-processes"]}))
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

func (w *Workflow) handleSet(c string) (err error) {

	cparts := strings.Fields(c)

	if len(cparts) < 3 {
		// Hmmm, malformated SET
		return fmt.Errorf("SET statement incomplete: '%s'\n", c)
	}

	vname := strings.Trim(cparts[1], "%") // nuke the lead/trail percents from the varname

	vvalue := strings.Join(cparts[2:len(cparts)], " ") // concatenate any end parts
	vvalue = strings.Trim(vvalue, "\"")                // nuke lead/trail dub-quotes
	vvalue = strings.Trim(vvalue, "'")                 // nuke lead/trail sing-quotes
	vvalue = w.varParse(vvalue)                        // Check it for vars

	if strings.Contains(vvalue, "S3(") {
		// We need a tokened S3 URL
		vvalue, err = w.handleS3(vvalue, GlobalVars["awsaccess_key"], GlobalVars["awsaccess_secretkey"])
	} else if strings.Contains(vvalue, "RAND(") {
		// We need a random string
		vvalue, err = w.handleRand(vvalue)
	}

	if _, ok := w.vars[vname]; ok {
		// already set, proceed but alert
		Debug.Printf("SET %s already set to '%s', now '%s'\n", vname, w.vars[vname], vvalue)
	}

	if err == nil {
		w.vars[vname] = vvalue
	}

	return
}

func (w *Workflow) handleS3(vvalue, accessKey, secretKey string) (string, error) {

	// Confirm we actually have the bits set
	if _, ok := GlobalVars["awsaccess_key"]; ok == false {
		return "", fmt.Errorf("No AWS access key set, but S3() called\n")
	} else if _, ok := GlobalVars["awsaccess_secretkey"]; ok == false {
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

func (w *Workflow) handleSleep(vvalue string) error {

	sleepFor, err := time.ParseDuration(vvalue)
	if err == nil {
		Debug.Printf("SLEEP for %s\n", vvalue)
		time.Sleep(sleepFor)
	}
	return err

}

// Get a saneMaxLimit, based on the number of commands in the workflow
func saneMaxLimitFromWorkflow(wf Workflow) int {
	// We need to count the commands, but factor out "SET" and "#"
	c := 0
	for _, command := range wf.Commands {
		if strings.HasPrefix(command, "SET ") || strings.HasPrefix(command, "#") {
			continue
		}
		c++
	}
	return saneMaxLimit(c)
}
