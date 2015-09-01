// +build go1.4

package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"strconv"
	"strings"
)

// CommandReturn is a structure returned after executing a command
type CommandReturn struct {
	Hostname string
	HostObj  Host
	Error    error
	Command  string
	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
	Quiet    bool
}

type Command struct {
	Cmd       string
	Host      Host
	SSHConfig *ssh.ClientConfig
	Sudo      bool
	Quiet     bool
}

type commandOut struct {
	Name    string
	Address string
	Command string
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
		Command: cr.Command,
	}

	if cr.Error != nil {
		f.Error = cr.Error.Error()
	}

	if strings.Contains(cr.Command, "needs-restarting") {
		plist := needsRestartingMangler(cr.StdoutStrings(true), makeList([]string{globalVars["dontrestart-processes"]}))
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
		fmt.Println("error:", err)
	}

	return j
}

// Process inspected the CommandReturn and outputs JSON
func (cr *CommandReturn) ToJSON() []byte {
	if cr.Quiet {
		return nil
	}

	f := cr.format()

	j, err := json.MarshalIndent(f, "\t", "\t")
	if err != nil {
		fmt.Println("error:", err)
	}

	return j
}

// Process inspected the CommandReturn and outputs structured text
func (cr *CommandReturn) ToText() {
	if cr.Quiet {
		return
	}

	f := cr.format()

	fmt.Printf("%s (%s): %s\n", f.Name, f.Address, f.Command)
	if len(f.Stdout) > 0 {
		fmt.Println("STDOUT:")
		for _, l := range f.Stdout {
			fmt.Println(l)
		}
	}
	if len(f.Stderr) > 0 {
		fmt.Println("STDERR:")
		for _, l := range f.Stderr {
			fmt.Println(l)
		}
	}
	fmt.Println("END\n")

}

func (c *Command) Exec() CommandReturn {

	var cr CommandReturn

	if c.Cmd == "" {
		log.Printf("Command Exec request has no Cmd!")
		cr.Error = fmt.Errorf("Command Exec request has no Cmd!")
		return cr
	}

	cmd := c.Cmd

	if c.Sudo {
		cmd = "sudo " + cmd
	}

	debugOut.Printf("Executing command '%s'\n", cmd)

	cr.HostObj = c.Host
	cr.Command = cmd
	cr.Quiet = c.Quiet
	cr.Error = nil

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

	conn, err := ssh.Dial("tcp", connectName+":"+port, c.SSHConfig)
	if err != nil {
		log.Printf("Connection to %s on port %s failed: %s\n", connectName, port, err)
		cr.Error = err
		return cr
	}

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
		if err := session.RequestPty("xterm", 80, 80, modes); err != nil {
			log.Printf("Request for pseudo terminal on %s failed: %s", connectName, err)
			cr.Error = err
			return cr
		}
	}

	// Set stdout/err to our byte buffers
	session.Stdout = &cr.Stdout
	session.Stderr = &cr.Stderr

	// Run the cmd
	err = session.Run(cmd)
	if err != nil {
		log.Printf("Execution of command failed on %s: %s", connectName, err)
		cr.Error = err
	}
	return cr

}

func serviceList(op string, list []string, res chan<- CommandReturn, com Command) {

	// sshd needs to restart first, completely, before other things fly
	for _, p := range list {

		if p == "sshd" {
			serviceCommand := "service " + p + " " + op + "; sleep 2"
			com.Cmd = serviceCommand
			res <- com.Exec()
			break
		}
	}

	for _, p := range list {
		if p == "sshd" {
			// Skip sshd, as we've already restarted it above, if appropriate
			continue
		}
		serviceCommand := "service " + p + " " + op + "; sleep 2"
		com.Cmd = serviceCommand
		go func(com Command) {
			res <- com.Exec()
		}(com)

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
