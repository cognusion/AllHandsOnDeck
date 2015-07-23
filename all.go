// +build go1.4

package main

import (
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
	Miscs     []Misc
}

func (c *Config) WorkflowIndex(workflow string) int {
	var flowIndex int = -1
	for i, wf := range c.Workflows {
		if wf.Name == workflow {
			flowIndex = i
			break
		}
	}
	return flowIndex
}

var debugOut *log.Logger

var globalVars map[string]string

func main() {

	var (
		sshAgent     bool
		sshKey       string
		configFolder string
		userName     string
		sudo         bool
		timeout      int
		cmd          string
		workflow     bool
		filter       string
		configTest   bool
		quiet        bool
		configDump   bool
		debug        bool

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
	flag.BoolVar(&configTest, "configtest", false, "Load and parse configs, and exit")
	flag.StringVar(&configFolder, "configs", "configs/", "Path to the folder where the config files are (*.json)")
	flag.StringVar(&userName, "user", currentUser.Username, "User to run as")
	flag.IntVar(&timeout, "timeout", 60, "Seconds before the entire operation times out")
	flag.BoolVar(&sudo, "sudo", false, "Whether to run commands via sudo")
	flag.BoolVar(&workflow, "workflow", false, "The --cmd is a workflow")
	flag.BoolVar(&quiet, "quiet", false, "Suppress most-if-not-all normal output")
	flag.BoolVar(&configDump, "configdump", false, "Load and parse configs, dump them to output and exit")
	flag.StringVar(&cmd, "cmd", "", "Command to run")
	flag.StringVar(&filter, "filter", "", "Boolean expression to positively filter on host elements (Tags, Name, Address, Arch, User, Port, etc.)")
	flag.Parse()

	if debug {
		debugOut = log.New(os.Stdout, "[DEBUG]", log.Lshortfile)
	} else {
		debugOut = log.New(ioutil.Discard, "", log.Lshortfile)
	}

	// Handle the configs
	if configFolder == "" {
		log.Fatalln("--configs must be set!")
	} else {
		// Load the conf object
		conf = loadConfigs(configFolder)

		// Build any needed global vars
		globalVars = miscToMap(conf.Miscs)
	}

	// We must have a command, no?
	if cmd == "" {
		log.Fatalln("--cmd must be set!")
	}

	// If cmd is a workflow
	//  - ensure the workflow exists
	//  - cache the location of the specified workflow
	if workflow {
		wfIndex = conf.WorkflowIndex(cmd)
		if wfIndex < 0 {
			log.Fatalf("Workflow '%s' does not exist in specified configs!\n", cmd)
		}
	}

	/*
	 * We are not allowing multiple keys, or key-per-hosts. If you need to possibly use
	 * multiple keys, ensure ssh-agent is running and has them added, and execute with
	 * --sshagent
	 */
	if sshAgent {
		// use SSH-Agent
		conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			log.Fatalf("Error connecting to the ssh-agent. It may not be running, or SSH_AUTH_SOCK may be set in this environment: %s\n", err)
		}
		defer conn.Close()
		ag := agent.NewClient(conn)
		auths = []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}
	} else {
		// Use a single key

		buf, err := ioutil.ReadFile(sshKey)
		if err != nil {
			log.Fatalf("Error reading specified key '%s': %s\n", sshKey, err)
		}
		key, err := ssh.ParsePrivateKey(buf)
		if err != nil {
			log.Fatalf("Error parsing specified key '%s': %s\n", sshKey, err)
		}
		auths = []ssh.AuthMethod{ssh.PublicKeys(key)}
	}

	if configDump {
		// Dump the config
		log.Println(conf)
		os.Exit(0)
	} else if configTest {
		// Just kicking the tires...
		log.Println("Config loaded and bootstrapped successfully...")
		os.Exit(0)
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

		debugOut.Printf("Host: %s\n", host.Name)

		// Handle alternate usernames
		configUser := userName
		if host.User != "" {
			configUser = host.User
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

			// Also, if there is a mintimeout, let's maybe use it
			if conf.Workflows[wfIndex].MinTimeout > timeout {
				timeout = conf.Workflows[wfIndex].MinTimeout
			}
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

				if quiet == false {
					// Process all of the enclosed CommandReturns
					for _, c := range res.CommandReturns {
						c.Process()
					}
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				log.Println("Workflow operation timed out!")
				return
			}
		} else {
			// Command
			select {
			case res := <-commandResults:
				if quiet == false {
					res.Process()
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				log.Println("Command operation timed out!")
				return
			}
		}
	}
}
