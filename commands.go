// +build go1.4

package main

import (
	"bytes"
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
}

// StdoutString return the Stdout buffer as a string
func (cr *CommandReturn) StdoutString() string {
	return cr.Stdout.String()
}

// StderrString return the Stderr buffer as a string
func (cr *CommandReturn) StderrString() string {
	return cr.Stderr.String()
}

// StdoutStrings return the Stdout buffer as a string array
func (cr *CommandReturn) StdoutStrings() []string {
	return strings.Split(cr.Stdout.String(), "\n")
}

// StderrStrings return the Stderr buffer as a string array
func (cr *CommandReturn) StderrStrings() []string {
	return strings.Split(cr.Stderr.String(), "\n")
}

// Process inspected the CommandReturn and outputs structured information about it
func (cr *CommandReturn) Process() {
	if strings.Contains(cr.Command, "needs-restarting") {
		plist := needsRestartingMangler(cr.StdoutStrings())
		log.Printf("%s: %s\n%v\n", cr.Hostname, cr.Command, plist)
	} else {
		log.Printf("%s: %s\n", cr.Hostname, cr.Command) 
		if cr.Stdout.Len() > 0 {
			log.Printf("STDOUT:\n%s\n", cr.StdoutString())
		}
		if cr.Stderr.Len() > 0 {
			log.Printf("STDERR:\n%s\n", cr.StderrString())
		}
		log.Println("END")
	}
}

func executeCommand(cmd string, host Host, config *ssh.ClientConfig, sudo bool) CommandReturn {

	if sudo {
		cmd = "sudo " + cmd
	}

	debugOut.Printf("Executing command '%s'\n", cmd)

	var cr CommandReturn
	cr.HostObj = host
	cr.Command = cmd
	cr.Error = nil

	var connectName string
	if host.Address != "" {
		connectName = host.Address
	} else {
		connectName = host.Name
	}
	cr.Hostname = connectName

	port := "22"
	if host.Port != 0 {
		port = strconv.Itoa(host.Port)
	}

	conn, err := ssh.Dial("tcp", connectName+":"+port, config)
	if err != nil {
		log.Printf("Connection to %s on port %s failed: %s\n", connectName, port, err)
		cr.Error = err
		return cr
	}

	session, _ := conn.NewSession()
	defer session.Close()

	if sudo {
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

func serviceList(op string, list []string, res chan<- CommandReturn, host Host, config *ssh.ClientConfig, sudo bool) {
	for _, p := range list {
		serviceCommand := "service " + p + " " + op

		go func(host Host) {
			res <- executeCommand(serviceCommand, host, config, sudo)
		}(host)

	}
}
