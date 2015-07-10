package main

import (
	"encoding/json"
	"flag"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"time"
)

type Config struct {
	Hosts     []Host
	Workflows []Workflow
}

func main() {

	var (
		sshAgent   bool
		sshKey     string
		configFile string
		userName   string
		sudo       bool
		timeout    int
		cmd        string
		workflow   bool
		filter     string
		debug      bool

		conf    Config
		auths   []ssh.AuthMethod
		wfIndex int
	)

	currentUser, _ := user.Current()

	commandResults := make(chan CommandReturn, 10)
	wfResults := make(chan WorkflowReturn, 10)

	flag.BoolVar(&sshAgent, "sshagent", false, "Connect and use SSH-Agent vs. user key")
	flag.StringVar(&sshKey, "sshkey", currentUser.HomeDir+"/.ssh/id_rsa", "If not using the SSH-Agent, where to grab the key")
	flag.BoolVar(&debug, "debug", false, "Enable Debug output")
	flag.StringVar(&configFile, "config", "", "Config file location to read and run from")
	flag.StringVar(&userName, "user", currentUser.Username, "User to run as")
	flag.IntVar(&timeout, "timeout", 5, "Seconds before command times out")
	flag.BoolVar(&sudo, "sudo", false, "Whether to run commands via sudo")
	flag.BoolVar(&workflow, "workflow", false, "The --cmd is a workflow")
	flag.StringVar(&cmd, "cmd", "", "Command to run")
	flag.StringVar(&filter, "filter", "", "Boolean expression to positively filter on Tags")
	flag.Parse()

	// Handle the configFile
	if configFile == "" {
		log.Fatalln("config must be set!")
	} else {
		buf, err := ioutil.ReadFile(configFile)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(buf, &conf)
		if err != nil {
			log.Fatal(err)
		}
	}

	// We must have a command, no?
	if cmd == "" {
		log.Fatalln("cmd must be set!")
	}

	// If cmd is a workflow
	//	- ensure the workflow exists
	//  - cache the location of the specified workflow
	if workflow {
		wfIndex = -1
		for i, wf := range conf.Workflows {
			if wf.Name == cmd {
				wfIndex = i
				break
			}
		}
		if wfIndex < 0 {
			log.Fatalf("Workflow '%s' does not exist in specified config!\n", cmd)
		}
	}

	/*
	 * We are no allowing multiple keys, or key-per-hosts. If you need to possibly use
	 * multiple keys, ensure ssh-agent is running and has them added, and execute with
	 * --sshagent
	 */
	if sshAgent {
		// use SSH-Agent
		conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		ag := agent.NewClient(conn)
		auths = []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}
	} else {
		// Use a single key

		buf, err := ioutil.ReadFile(sshKey)
		if err != nil {
			log.Fatal(err)
		}
		key, err := ssh.ParsePrivateKey(buf)
		if err != nil {
			log.Fatal(err)
		}
		auths = []ssh.AuthMethod{ssh.PublicKeys(key)}
	}

	hostCount := len(conf.Hosts)
	for _, host := range conf.Hosts {

		// Check to see if the this host matches our filter
		if filter != "" && host.If(filter) == false {
			hostCount--
			continue
		}

		// Additionally, if there is a filter on the workflow, check the host against that too.
		if workflow && conf.Workflows[wfIndex].Filter != "" && host.If(conf.Workflows[wfIndex].Filter) == false {
			hostCount--
			continue
		}

		//log.Printf("Host: %s\n",host.Name)

		// Handle alternate usernames
		configUser := userName
		if host.AltUser != "" {
			configUser = host.AltUser
		}

		// SSH Config
		config := &ssh.ClientConfig{
			User: configUser,
			Auth: auths,
		}

		/*
		 * This is where the work is getting accomplished.
		 *   Workflows are configured sets of commands and logics, with sets of returns
		 *   Commands are single directives, with single returns
		 */
		if workflow {
			// Workflow

			go func(host Host) {
				wfResults <- conf.Workflows[wfIndex].Exec(host, config, sudo)
			}(host)

		} else {
			// Command
			go func(host Host) {
				commandResults <- executeCommand(cmd, host, config, sudo)
			}(host)
		}
	}

	// We wait for all the goros to finish up
	for i := 0; i < hostCount; i++ {
		if workflow {
			// Workflow
			select {
			case res := <-wfResults:
				if res.Completed == false {
					log.Printf("Workflow %s did not fully complete\n", res.Name)
				}
				for _, c := range res.CommandReturns {
					c.Process()
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				log.Println("Timed out!")
				return
			}
		} else {
			// Command
			select {
			case res := <-commandResults:
				res.Process()
			case <-time.After(time.Duration(timeout) * time.Second):
				log.Println("Timed out!")
				return
			}
		}

	}
}
