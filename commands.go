package main

import (
	"bytes"
	"golang.org/x/crypto/ssh"
	"log"
	"strings"
	"strconv"
)

type CommandReturn struct {
	Hostname string
	HostObj  Host
	Error    error
	Command  string
	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
}

func (cr *CommandReturn) StdoutString() string {
	return cr.Stdout.String()
}

func (cr *CommandReturn) StderrString() string {
	return cr.Stderr.String()
}

func (cr *CommandReturn) StdoutStrings() []string {
	return strings.Split(cr.Stdout.String(),"\n")
}

func (cr *CommandReturn) StderrStrings() []string {
	return strings.Split(cr.Stderr.String(),"\n")
}

func (cr *CommandReturn) Process() {
	if strings.Contains(cr.Command, "needs-restarting") {
		plist := plistToInits(cr.StdoutStrings())
		log.Printf("%s: %s\n%v\n",cr.Hostname,cr.Command,plist)	
	} else {
		log.Printf("%s: %s\nSTDOUT:\n%s\nSTDERR:\n%s\nEND\n", cr.Hostname, cr.Command, cr.StdoutString(), cr.StderrString())
	}
}

func executeCommand(cmd string, host Host, config *ssh.ClientConfig, sudo bool) CommandReturn {

	if sudo {
		cmd = "sudo " + cmd
	}
	
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

	conn, err := ssh.Dial("tcp", connectName+":"+strconv.Itoa(host.Port), config)
	if err != nil {
		log.Printf("Dial for %s failed",connectName+":"+strconv.Itoa(host.Port))
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
		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		    log.Printf("request for pseudo terminal failed: %s", err)
		    cr.Error = err
			return cr
		}
	}

	cr.Hostname = connectName
	cr.HostObj = host
	session.Stdout = &cr.Stdout
	session.Stderr = &cr.Stderr
	err = session.Run(cmd)
	if err != nil {
		log.Printf("execution of command failed: %s", err)
		cr.Error = err
	}
	return cr

}