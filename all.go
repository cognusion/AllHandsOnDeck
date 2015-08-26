// +build go1.4

/*
All Hands On Deck (aka "all") is a simple agentless orchestration system written
in Go, for Linux. You can run it *from* any platform that supports Go (Macs are
popular, I hear). Commands are executed in parallelish, as are workflows (commands
within a workflow are executed serially)

Usage of ./all:
  -cmd string
    	Command to run
  -configdump
    	Load and parse configs, dump them to output and exit
  -configs string
    	Path to the folder where the config files are (*.json) (default "configs/")
  -configtest
    	Load and parse configs, and exit
  -debug
    	Enable Debug output
  -filter string
    	Boolean expression to positively filter on host elements (Tags, Name, Address, Arch, User, Port, etc.)
  -listhosts
    	List the hostnames and addresses and exit
  -listworkflows
    	List the workflows and exit
  -max int
    	Specify the maximum number of concurent commands to execute. (default 15)
  -quiet
    	Suppress most-if-not-all normal output
  -sshagent
    	Connect and use SSH-Agent vs. user key
  -sshkey string
    	If not using the SSH-Agent, where to grab the key (default "/Users/M/.ssh/id_rsa")
  -sudo
    	Whether to run commands via sudo
  -timeout int
    	Seconds before the entire operation times out (default 60)
  -user string
    	User to run as (default "M")
  -wave int
    	Specify which "wave" this should be applied to
  -workflow
    	The --cmd is a workflow
*/
package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"time"
)

// Config is a toplevel struct to house arrays of Hosts, Workflows, and Miscs
type Config struct {
	Hosts     []Host
	Workflows []Workflow
	Miscs     []Misc
}

// Merge properly merges the provided Config, into the parent Config
func (c *Config) Merge(conf Config) {
	c.Hosts = append(c.Hosts, conf.Hosts...)
	c.Workflows = append(c.Workflows, conf.Workflows...)
	c.Miscs = append(c.Miscs, conf.Miscs...)
}

// WorkflowIndex finds the named workflow in the Config, and
// returns its index, or -1 if it is not found
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

var debugOut *log.Logger = log.New(ioutil.Discard, "", log.Lshortfile)

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
		listHosts    bool
		listFlows    bool
		debug        bool
		wave         int
		max          int

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
	flag.BoolVar(&listHosts, "listhosts", false, "List the hostnames and addresses and exit")
	flag.BoolVar(&listFlows, "listworkflows", false, "List the workflows and exit")
	flag.IntVar(&wave, "wave", 0, "Specify which \"wave\" this should be applied to")
	flag.IntVar(&max, "max", 15, "Specify the maximum number of concurent commands to execute.")
	flag.Parse()

	if debug {
		debugOut = log.New(os.Stdout, "[DEBUG]", log.Lshortfile)
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

	/*
	 * Any "miscs" config stuff here
	 *
	 */
	if _, ok := globalVars["usesshagent"]; ok && globalVars["usesshagent"] == "true" {
		sshAgent = true
	}

	if _, ok := globalVars["maxexecs"]; ok {
		m, err := strconv.Atoi(globalVars["maxexecs"])
		if err != nil {
			log.Fatalf("maxexecs set to '%s', and cannot convert to number: %s\n", globalVars["maxexecs"], err.Error())
		}
		max = m
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
		fmt.Println(dumpConfigs(conf))
		os.Exit(0)
	} else if configTest {
		// Just kicking the tires...
		fmt.Println("Config loaded and bootstrapped successfully...")
		os.Exit(0)
	} else if listHosts {
		for _, host := range conf.Hosts {
			if host.Offline == true {
				continue
			} else if filter != "" && host.If(filter) == false {
				// Check to see if the this host matches our filter
				continue
			} else if wave != 0 && host.Wave != wave {
				// Check to see if we're using waves, and if this is in it
				continue
			}
			fmt.Printf("%s: %s\n", host.Name, host.Address)
		}
		os.Exit(0)
	} else if listFlows {
		for _, flow := range conf.Workflows {
			fmt.Printf("%s\n", flow.Name)
		}
		os.Exit(0)
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

	// To keep things sane, we gate the number of goros that can be executing remote
	// commands to a limit.
	sem := NewSemaphore(max)

	// We've made it through checks and tests.
	// Let's do this.
	hostList := make(map[string]bool)
	for _, host := range conf.Hosts {

		// Check to see if the host is offline
		if host.Offline == true {
			continue
		}

		// Check to see if we're using waves, and if this is in it
		if wave != 0 && host.Wave != wave {
			continue
		}

		// Check to see if the this host matches our filter
		if filter != "" && host.If(filter) == false {
			continue
		}

		// Additionally, if there is a filter on the workflow, check the host against that too.
		if workflow && conf.Workflows[wfIndex].Filter != "" && host.If(conf.Workflows[wfIndex].Filter) == false {
			continue
		}

		// Add the host to the list, and set its return status to false
		hostList[host.Name] = false
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

		com := &Command{Host: host, SSHConfig: config, Sudo: sudo}

		if workflow {
			// Workflow
			go func(com Command) {
				sem.Lock()
				defer sem.Unlock()

				wfResults <- conf.Workflows[wfIndex].Exec(com)
			}(*com)

			// Also, if there is a mintimeout, let's maybe use it
			if conf.Workflows[wfIndex].MinTimeout > timeout {
				timeout = conf.Workflows[wfIndex].MinTimeout
			}
		} else {
			// Command
			com.Cmd = cmd
			go func(com Command) {
				sem.Lock()
				defer sem.Unlock()

				commandResults <- com.Exec()
			}(*com)
		}
	}

	// We wait for all the goros to finish up
	for i := 0; i < len(hostList); i++ {
		if workflow {
			// Workflow
			select {
			case res := <-wfResults:
				hostList[res.HostObj.Name] = true // returned is good enough for this

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
				var badHosts []string
				for h, v := range hostList {
					if v == false {
						badHosts = append(badHosts, h)
					}
				}
				log.Printf("Workflow operation timed out! The following hosts haven't returned: %s\n", badHosts)
				return
			}
		} else {
			// Command
			select {
			case res := <-commandResults:
				hostList[res.HostObj.Name] = true // returned is good enough for this

				if quiet == false {
					res.Process()
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				var badHosts []string
				for h, v := range hostList {
					if v == false {
						badHosts = append(badHosts, h)
					}
				}
				log.Printf("Command operation timed out! The following hosts haven't returned: %s\n", badHosts)
				return
			}
		}
	}
}
