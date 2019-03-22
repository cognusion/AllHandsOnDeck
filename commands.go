// +build go1.4

package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strconv"
	"strings"
	"time"
)

// CommandReturn is a structure returned after executing a Command
type CommandReturn struct {
	Hostname string
	HostObj  Host
	Error    error
	Command  string
	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
	Quiet    bool
}

// Command is a structure to hold the necessary info to execute
// a command
type Command struct {
	Cmd       string
	Host      Host
	SSHConfig *ssh.ClientConfig
	Sudo      bool
	Quiet     bool
}

// commandOut is a helper struct to allow easier formating
// of CommandResults for output
type commandOut struct {
	Name    string
	Address string
	Command string
	Date    time.Time
	Stdout  []string
	Stderr  []string
	Error   string
}

// StdoutString return the Stdout buffer as a string
func (cr *CommandReturn) StdoutString(nullToSpace bool) string {
	if nullToSpace {
		s := bytes.Replace(cr.Stdout.Bytes(), []byte{00}, []byte(" "), -1)
		return string(s)
	} else {
		return cr.Stdout.String()
	}
}

// StderrString return the Stderr buffer as a string
func (cr *CommandReturn) StderrString(nullToSpace bool) string {
	if nullToSpace {
		s := bytes.Replace(cr.Stderr.Bytes(), []byte{00}, []byte(" "), -1)
		return string(s)
	} else {
		return cr.Stderr.String()
	}
}

// StdoutStrings return the Stdout buffer as a string array
func (cr *CommandReturn) StdoutStrings(nullToSpace bool) []string {
	s := strings.Split(cr.StdoutString(nullToSpace), "\n")
	return s[:len(s)-1]
}

// StderrStrings return the Stderr buffer as a string array
func (cr *CommandReturn) StderrStrings(nullToSpace bool) []string {
	s := strings.Split(cr.StderrString(nullToSpace), "\n")
	return s[:len(s)-1]
}

// Process inspected the CommandReturn pre-format the output
func (cr *CommandReturn) format() commandOut {

	f := commandOut{
		Name:    cr.HostObj.Name,
		Address: cr.HostObj.Address,
		Date:    time.Now(),
		Command: cr.Command,
	}

	if cr.Error != nil {
		f.Error = cr.Error.Error()
	}

	if strings.Contains(cr.Command, "needs-restarting") {
		plist := needsRestartingMangler(cr.StdoutStrings(true), makeList([]string{GlobalVars["dontrestart-processes"]}))
		f.Stdout = []string{"Restart list:"}
		f.Stdout = append(f.Stdout, plist...)
	} else {
		if cr.Stdout.Len() > 0 {
			f.Stdout = cr.StdoutStrings(false)
		}
		if cr.Stderr.Len() > 0 {
			f.Stderr = cr.StderrStrings(false)
		}
	}

	return f
}

// Process inspected the CommandReturn and outputs XML
func (cr *CommandReturn) ToXML() []byte {
	if cr.Quiet {
		return nil
	}

	f := cr.format()

	j, err := xml.Marshal(f)
	if err != nil {
		Error.Println("error:", err)
	}

	return j
}

// Process inspected the CommandReturn and outputs JSON
func (cr *CommandReturn) ToJSON(pretty bool) (j []byte) {
	if cr.Quiet {
		return
	}

	f := cr.format()

	var err error
	if pretty == false {
		j, err = json.Marshal(f)
	} else {
		j, err = json.MarshalIndent(f, "", "\t")
	}

	if err != nil {
		Error.Println("error:", err)
	}

	return
}

// Process inspected the CommandReturn and outputs structured text
func (cr *CommandReturn) ToText() (out string) {
	if cr.Quiet {
		return
	}

	f := cr.format()

	out = out + fmt.Sprintf("%s (%s): %s\n", f.Name, f.Address, f.Command)
	if len(f.Stdout) > 0 {
		out = out + "STDOUT:\n"
		for _, l := range f.Stdout {
			out = out + l + "\n"
		}
	}
	if len(f.Stderr) > 0 {
		out = out + "STDERR:\n"
		for _, l := range f.Stderr {
			out = out + l + "\n"
		}
	}
	out = out + "END\n"

	return
}

// Execute the Command structure, returning a CommandReturn
func (c *Command) Exec() (cr CommandReturn) {

	if c.Cmd == "" {
		Error.Printf("Command Exec request has no Cmd!")
		cr.Error = fmt.Errorf("Command Exec request has no Cmd!")
		return cr
	}

	cmd := c.Cmd

	// Do we need to prepend sudo?
	if c.Sudo {
		cmd = "sudo " + cmd
	}

	Debug.Printf("Executing command '%s'\n", cmd)

	cr = CommandReturn{
		HostObj: c.Host,
		Command: cmd,
		Quiet:   c.Quiet,
		Error:   nil,
	}

	var connectName string
	if c.Host.Address != "" {
		connectName = c.Host.Address
	} else {
		connectName = c.Host.Name
	}
	cr.Hostname = connectName

	port := "22"
	if c.Host.Port != 0 {
		port = strconv.Itoa(c.Host.Port)
	}

	if _, ok := GlobalVars["dryrun"]; ok == false {
		// We're doing it live

		conn, err := ssh.Dial("tcp", connectName+":"+port, c.SSHConfig)
		if err != nil {
			Error.Printf("Connection to %s on port %s failed: %s\n", connectName, port, err)
			cr.Error = err
			return
		}
		defer conn.Close()

		session, _ := conn.NewSession()
		defer session.Close()

		if c.Sudo {
			// Set up terminal modes
			modes := ssh.TerminalModes{
				ssh.ECHO:          0,     // disable echoing
				ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
				ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
			}
			// Request pseudo terminal
			if serr := session.RequestPty("xterm", 80, 80, modes); serr != nil {
				Error.Printf("Request for pseudo terminal on %s failed: %s", connectName, serr)
				cr.Error = serr
				return
			}
		}

		// Set stdout/err to our byte buffers
		session.Stdout = &cr.Stdout
		session.Stderr = &cr.Stderr

		// Run the cmd
		err = session.Run(cmd)
		if err != nil {
			Error.Printf("Execution of command failed on %s: %s", connectName, err)
			cr.Error = err
		}
	}

	return

}

// Given a list of services to operate on, Do The Right Thing
func serviceList(op string, list []string, res chan<- CommandReturn, com Command) {

	// sshd needs to restart first, completely, before other things fly
	if op == "restart" {
		for _, p := range list {
			if p == "sshd" {
				serviceCommand := "service " + p + " " + op + "; sleep 2"
				com.Cmd = serviceCommand
				res <- com.Exec()
				break
			}
		}
	}

	for _, p := range list {
		if p == "sshd" {
			// Skip sshd, as we've already restarted it above, if appropriate
			// or will stop it below, if appropriate
			continue
		}
		com.Cmd = "service " + p + " " + op + "; sleep 2"

		// We're executing these concurrently
		go func(com Command) {
			res <- com.Exec()
		}(com)

	}

	// sshd needs to stop first, completely, after everything else
	if op == "stop" {
		for _, p := range list {
			if p == "sshd" {
				serviceCommand := "service " + p + " " + op + "; sleep 2"
				com.Cmd = serviceCommand
				res <- com.Exec()
				break
			}
		}
	}
}

/*
func scp(sPath, dPath string, com Command) error {

	session, err := com.SSHConfig.connect()
	if err != nil {
		return err
	}
	defer session.Close()

	src, srcErr := os.Open(sPath)
	if srcErr != nil {
		return srcErr
	}

	srcStat, statErr := src.Stat()
	if statErr != nil {
		return statErr
	}

	go func() {
		w, _ := session.StdinPipe()

		fmt.Fprintln(w, "C0644", srcStat.Size(), dPath)

		if srcStat.Size() > 0 {
			io.Copy(w, src)
			fmt.Fprint(w, "\x00")
			w.Close()
		} else {
			fmt.Fprint(w, "\x00")
			w.Close()
		}
	}()

	if err := session.Run(fmt.Sprintf("scp -t %s", dPath)); err != nil {
		return err
	}

	return nil
}
*/
