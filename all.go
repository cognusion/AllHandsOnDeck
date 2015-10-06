// +build go1.4

/*
All Hands On Deck (aka "all") is a simple agentless orchestration system written
in Go, for Linux. You can run it *from* any platform that supports Go (Macs are
popular, I hear). Commands are executed in parallelish, as are workflows (commands
within a workflow are executed serially)
*/
package main

import (
	"flag"
	"fmt"
	"github.com/cheggaaa/pb"
	"github.com/cognusion/semaphore"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"time"
)

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
		format       string
		logFile      string
		errorLogFile string
		debugLogFile string
		progressBar  bool

		conf    Config
		auths   []ssh.AuthMethod
		wfIndex int
	)

	// Grab the current username, best we can
	currentUser, _ := user.Current()

	// Channels for command and workflow -results
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
	flag.IntVar(&max, "max", 0, "Specify the maximum number of concurent commands to execute. Set to 0 to make a good guess for you (default 0)")
	flag.StringVar(&format, "format", "text", "Output format. One of: text, json, or xml")
	flag.StringVar(&logFile, "logfile", "", "Output to a logfile, instead of standard out (enables progressbar to screen)")
	flag.StringVar(&errorLogFile, "errorlogfile", "", "Output errors to a logfile, instead of standard error")
	flag.StringVar(&debugLogFile, "debuglogfile", "", "Output debugs to a logfile, instead of standard error")
	flag.BoolVar(&progressBar, "bar", true, "If outputting to a logfile, display a progress bar")
	flag.Parse()

	/*
	 * Initially handle Logging: debug, error, and "standard"
	 * HINT: We do this again after the configs have been solidified
	 */
	if debug {
		SetDebug(debugLogFile)
	}
	if logFile != "" {
		SetLog(logFile)
	}
	if errorLogFile != "" {
		SetError(errorLogFile)
	}

	/*
	 * Handle the configs
	 *
	 */
	if configFolder == "" {
		log.Fatalln("--configs must be set!")
	} else {
		// Load the conf object from the config
		// files in the configFolder
		conf = loadConfigs(configFolder)

		// Build any needed global vars
		GlobalVars = miscToMap(conf.Miscs)
	}

	/*
	 * Any "miscs" config stuff here
	 *
	 */
	if _, ok := GlobalVars["usesshagent"]; ok && GlobalVars["usesshagent"] == "true" {
		sshAgent = true
	}

	if _, ok := GlobalVars["maxexecs"]; ok {
		m, err := strconv.Atoi(GlobalVars["maxexecs"])
		if err != nil {
			log.Fatalf("maxexecs set to '%s', and cannot convert to number: %s\n", GlobalVars["maxexecs"], err.Error())
		}
		max = m
	}

	if f, ok := GlobalVars["outputformat"]; ok {
		format = f
	}

	if l, ok := GlobalVars["outputlog"]; ok {
		logFile = l
		SetLog(l)
	}

	if l, ok := GlobalVars["erroroutputlog"]; ok {
		errorLogFile = l
		SetError(l)
	}

	if l, ok := GlobalVars["debugoutputlog"]; ok {
		debugLogFile = l
		SetDebug(l)
	}

	/*
	 * All the breaking options here
	 * Use "fmt" in lieu of "Log" for
	 * stdout
	 */
	if configDump {
		// Dump the config
		fmt.Println(dumpConfigs(conf))
		os.Exit(0)
	} else if configTest {
		// Just kicking the tires...
		fmt.Println("Config loaded and bootstrapped successfully...")
		os.Exit(0)
	} else if listHosts {
		// List all the configured hosts, and exit
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
		// List all the configured workflows, and exit
		for _, flow := range conf.Workflows {
			fmt.Printf("%s\n", flow.Name)
		}
		os.Exit(0)
	}

	/*
	 * Syntax checks here
	 *
	 */

	// We must have a command, no?
	if cmd == "" {
		log.Fatalln("--cmd must be set!")
	}

	// Constrain format
	if format != "text" && format != "json" && format != "xml" {
		log.Fatalln(`format must be one of "text", "json", or "xml"`)
	}

	/*
	 * We are not allowing multiple keys, or key-per-hosts. If you need to possibly use
	 * multiple keys, ensure ssh-agent is running and has them added, and execute with
	 * --sshagent
	 *
	 * TODO: Windows?
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

	// If cmd is a workflow
	//  - ensure the workflow exists
	//  - cache the location of the specified workflow
	//  - Do The Right Thing
	if workflow {
		wfIndex = conf.WorkflowIndex(cmd)
		if wfIndex < 0 {
			log.Fatalf("Workflow '%s' does not exist in specified configs!\n", cmd)
		}

		if max == 0 {
			// Autoconfig max execs
			max = saneMaxLimitFromWorkflow(conf.Workflows[wfIndex])
		}

		// Init the workflow
		conf.Workflows[wfIndex].Init()

	} else {
		// cmd is not a workflow
		if max == 0 {
			// Autoconfig max execs
			max = saneMaxLimit(1)
		}
	}

	// Autoconfig based on GOMAXPROCS (lame)
	if max == -1 {
		max = runtime.GOMAXPROCS(0)
	}

	// To keep things sane, we gate the number of goros that can be executing remote
	// commands to a limit.
	Debug.Printf("Max simultaneous execs set to %d\n", max)
	sem := semaphore.NewSemaphore(max)

	// Status bar! Hosts * 2 because we have the exec phase,
	// and then the collection phase
	bar := pb.New(len(conf.Hosts) * 2)

	if progressBar && logFile != "" {
		Debug.Printf("BAR: Set to %d\n", len(conf.Hosts)*2)
		bar.Start()
	}

	// We've made it through checks and tests.
	// Let's do this.
	hostList := make(map[string]bool)
	for _, host := range conf.Hosts {

		// Check to see if the host is offline
		if host.Offline == true {
			bar.Increment()
			continue
		}

		// Check to see if we're using waves, and if this is in it
		if wave != 0 && host.Wave != wave {
			bar.Increment()
			continue
		}

		// Check to see if the this host matches our filter
		if filter != "" && host.If(filter) == false {
			bar.Increment()
			continue
		}

		// Additionally, if there is a filter on the workflow, check the host against that too.
		if workflow && conf.Workflows[wfIndex].Filter != "" && host.If(conf.Workflows[wfIndex].Filter) == false {
			bar.Increment()
			continue
		}

		// Add the host to the list, and set its return status to false
		hostList[host.Name] = false
		Debug.Printf("Host: %s\n", host.Name)

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

		com := Command{Host: host, SSHConfig: config, Sudo: sudo}

		if workflow {
			// Workflow
			go func() {
				defer bar.Increment()
				sem.Lock()
				defer sem.Unlock()

				wfResults <- conf.Workflows[wfIndex].Exec(com)

			}()

			// Also, if there is a mintimeout, let's maybe use it
			if conf.Workflows[wfIndex].MinTimeout > timeout {
				timeout = conf.Workflows[wfIndex].MinTimeout
			}
		} else {
			// Command
			com.Cmd = cmd
			go func() {
				defer bar.Increment()
				sem.Lock()
				defer sem.Unlock()

				commandResults <- com.Exec()
			}()
		}
	}

	/*
	 * Post-run, pre-result cleanups
	 *
	 */
	if progressBar {
		// It's highly likely that we filtered out a bunch of hosts, and
		// while we incremented the bar as those came along, we still have
		// the response pass to reconcile
		Debug.Printf("BAR: Catching up on %d\n", len(conf.Hosts)-len(hostList))
		bar.Add(len(conf.Hosts) - len(hostList))
	}

	// We wait for all the goros to finish up
	for i := 0; i < len(hostList); i++ {
		if workflow {
			// Workflow
			select {
			case res := <-wfResults:
				hostList[res.HostObj.Name] = true // returned is good enough for this
				bar.Increment()

				if res.Completed == false {
					Error.Printf("Workflow %s did not fully complete\n", res.Name)
				}

				if quiet == false {
					// Process all of the enclosed CommandReturns

					for _, c := range res.CommandReturns {
						if c.Quiet {
							continue
						}
						switch format {
						case "text":
							Log.Println(c.ToText())
						case "xml":
							Log.Println(string(c.ToXML()))
						case "json":
							Log.Println(string(c.ToJSON(false)))
						}
					}

				}
			case <-time.After(time.Duration(timeout) * time.Second):
				var badHosts []string
				for h, v := range hostList {
					if v == false {
						badHosts = append(badHosts, h)
						bar.Increment()
					}
				}
				Error.Printf("Workflow operation timed out! The following hosts haven't returned: %s\n", badHosts)
				return
			}
		} else {
			// Command
			select {
			case res := <-commandResults:
				hostList[res.HostObj.Name] = true // returned is good enough for this
				bar.Increment()

				if quiet == false && res.Quiet == false {
					switch format {
					case "text":
						Log.Println(res.ToText())
					case "xml":
						Log.Println(string(res.ToXML()))
					case "json":
						Log.Println(string(res.ToJSON(false)))
					}
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				var badHosts []string
				for h, v := range hostList {
					if v == false {
						badHosts = append(badHosts, h)
						bar.Increment()
					}
				}
				Error.Printf("Command operation timed out! The following hosts haven't returned: %s\n", badHosts)
				return
			}
		}
	}

	/*
	 * Post-result, pre-exit cleanups
	 *
	 */
	if progressBar && logFile != "" {
		bar.Finish()
	}
}
