//go:build go1.8

/*
All Hands On Deck (aka "all") is a simple agentless orchestration system written
in Go, for Linux. You can run it *from* any platform that supports Go (Macs are
popular, I hear). Commands are executed in parallelish, as are workflows (commands
within a workflow are executed serially)
*/
package main

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/cognusion/semaphore"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
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
		workflow     string
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
		dryrun       bool
		sleepStr     string
		awsHosts     bool
		awsRegions   string
		cliVars      string
		dnf          bool

		conf     Config
		auths    []ssh.AuthMethod
		wfIndex  int
		sleepFor time.Duration
		wg       sync.WaitGroup
	)

	// Grab the current username, best we can
	currentUser, _ := user.Current()

	// Channels for command and workflow -results
	commandResults := make(chan CommandReturn, 100)
	wfResults := make(chan WorkflowReturn, 100)

	pflag.BoolVar(&sshAgent, "sshagent", false, "Connect and use SSH-Agent vs. user key")
	pflag.StringVar(&sshKey, "sshkey", currentUser.HomeDir+"/.ssh/id_rsa", "If not using the SSH-Agent, where to grab the key")
	pflag.BoolVar(&debug, "debug", false, "Enable Debug output")
	pflag.BoolVar(&configTest, "configtest", false, "Load and parse configs, and exit")
	pflag.StringVar(&configFolder, "configs", "configs/", "Path to the folder where the config files are (*.json)")
	pflag.StringVar(&userName, "user", currentUser.Username, "User to run as")
	pflag.IntVar(&timeout, "timeout", 60, "Seconds before the entire operation times out")
	pflag.BoolVar(&sudo, "sudo", false, "Whether to run commands via sudo")
	pflag.StringVar(&workflow, "workflow", "", "The workflow to run")
	pflag.BoolVar(&quiet, "quiet", false, "Suppress most-if-not-all normal output")
	pflag.BoolVar(&configDump, "configdump", false, "Load and parse configs, dump them to output and exit")
	pflag.StringVar(&cmd, "cmd", "", "The command to run")
	pflag.StringVar(&filter, "filter", "", "Boolean expression to positively filter on host elements (Tags, Name, Address, Arch, User, Port, etc.)")
	pflag.BoolVar(&listHosts, "listhosts", false, "List the hostnames and addresses and exit")
	pflag.BoolVar(&listFlows, "listworkflows", false, "List the workflows and exit")
	pflag.IntVar(&wave, "wave", 0, "Specify which \"wave\" this should be applied to")
	pflag.IntVar(&max, "max", 0, "Specify the maximum number of concurrent commands to execute. Set to 0 to make a good guess for you (default 0)")
	pflag.StringVar(&format, "format", "text", "Output format. One of: text, json, or xml")
	pflag.StringVar(&logFile, "logfile", "", "Output to a logfile, instead of standard out (enables progressbar to screen)")
	pflag.StringVar(&errorLogFile, "errorlogfile", "", "Output errors to a logfile, instead of standard error")
	pflag.StringVar(&debugLogFile, "debuglogfile", "", "Output debugs to a logfile, instead of standard error")
	pflag.BoolVar(&progressBar, "bar", true, "If outputting to a logfile, display a progress bar")
	pflag.BoolVar(&dryrun, "dryrun", false, "If you want to go through the motions, but never actually SSH to anything")
	pflag.StringVar(&sleepStr, "sleep", "0ms", "Duration to sleep between host iterations (e.g. 32ms or 1s)")
	pflag.BoolVar(&awsHosts, "awshosts", false, "Get EC2 hosts and tags from AWS API")
	pflag.StringVar(&awsRegions, "awsregions", "", "Comma-delimited list of AWS Regions to check if --awshosts is set")
	pflag.StringVar(&cliVars, "vars", "", "Comma-delimited list of variables to pass in for use in workflows, sometimes")
	pflag.BoolVar(&dnf, "dnf", false, "Use dnf instead of yum for some commands")
	pflag.Parse()

	/*
	 * Initially handle Logging: debug, error, and "standard"
	 * HINT: We do this again after the configs have been solidified
	 */
	if debug {
		SetDebug(debugLogFile)
	}
	if logFile != "" && logFile != "STDOUT" {
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

	if listFlows {
		// List all the configured workflows, and exit
		for _, flow := range conf.Workflows {
			if debug {
				flow.Init()
				fmt.Printf("%s\n%#v\n\n", flow.Name, flow)
			} else {
				fmt.Printf("%s\n", flow.Name)
			}
		}
		os.Exit(0)
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
		if logFile != "STDOUT" {
			format = f
		}
	}

	if l, ok := GlobalVars["outputlog"]; ok {
		if logFile != "STDOUT" {
			logFile = l
			SetLog(l)
		}
	}

	if l, ok := GlobalVars["erroroutputlog"]; ok {
		errorLogFile = l
		SetError(l)
	}

	if l, ok := GlobalVars["debugoutputlog"]; ok {
		debugLogFile = l
		SetDebug(l)
	}

	if _, ok := GlobalVars["useawshosts"]; ok && GlobalVars["useawshosts"] == "true" {
		awsHosts = true
	}

	if r, ok := GlobalVars["awsregions"]; ok && awsRegions == "" {
		awsRegions = r
	}

	if _, ok := GlobalVars["usednf"]; ok && GlobalVars["usednf"] == "true" {
		dnf = true
	}

	/*
	 * Are we dealing with AWS hosts/tags/regions?
	 */
	if awsHosts {
		var regions []string
		if awsRegions != "" {
			// CLI
			regions = strings.Split(awsRegions, ",")
		} else if r, ok := GlobalVars["aws_regions"]; ok {
			// Misc
			regions = strings.Split(r, ",")
		} else {
			// Grab default
			regions = append(regions, getAwsRegion())
		}

		// Grab the keys from the environment
		var aKey, sKey string

		// Access key
		if k, ok := GlobalVars["awsaccess_key"]; ok {
			aKey = k
		} else {
			aKey = os.Getenv("AWS_ACCESS_KEY_ID")
		}

		// Secret key
		if k, ok := GlobalVars["awsaccess_secretkey"]; ok {
			sKey = k
		} else {
			sKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		}

		var awsconf Config
		for _, region := range regions {
			session := initAWS(region, aKey, sKey)
			resp, err := getEc2Instances(session)

			if err != nil {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())

			} else {
				for idx := range resp.Reservations {
					for _, inst := range resp.Reservations[idx].Instances {
						if inst.PrivateIpAddress == nil {
							// Stopped, terminated, whatevs.
							continue
						}
						if inst.Platform == nil || *inst.Platform != "windows" {
							// Not Windows, phew
							awsconf.AddHost(newHostFromInstance(inst))
						}
					}
				}
			}
			// grab all the hosts
			// populate the Config
		}
		conf.Merge(awsconf)
	}

	/*
	 * If we have a workflow,
	 * and cmd is a list,
	 * we handle that here
	 */
	if workflow != "" && strings.Contains(workflow, ",") {
		newFlow := Workflow{
			Name:      workflow,
			MustChain: false,
		}

		for _, c := range strings.Split(workflow, ",") {
			wfi := conf.WorkflowIndex(c)
			if wfi < 0 {
				log.Fatalf("Workflow '%s' in chain does not exist in specified configs!\n", c)
			}
			newFlow.Merge(&conf.Workflows[wfi])
		}

		conf.Workflows = append(conf.Workflows, newFlow)

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
		// List all the configured hosts, applying filtering logic, and exit
		if workflow != "" {
			wfIndex = conf.WorkflowIndex(workflow)
		} else {
			wfIndex = -1
		}

		filteredHosts := conf.FilteredHostList(filter, wave, wfIndex)

		for _, host := range filteredHosts {
			fmt.Printf("%s: %s\n", host.Name, host.Address)
		}
		os.Exit(0)
	}

	/*
	 * Syntax checks here
	 *
	 */

	// Can't do both, anymore!
	if cmd != "" && workflow != "" {
		log.Fatalln("--cmd and --workflow are mutually exclusive!")
	}

	// We must have a command, no?
	if cmd == "" && workflow == "" {
		log.Fatalln("--cmd or --workflow must be set!")
	}

	// Detect param swallowing
	if strings.HasPrefix(cmd, "-") {
		log.Fatalf("--cmd looks to be swallowing parameter '%s'\n", cmd)
	}

	if strings.HasPrefix(workflow, "-") {
		log.Fatalf("--workflow looks to be swallowing parameter '%s'\n", workflow)
	}

	// Constrain format
	if format != "text" && format != "json" && format != "xml" {
		log.Fatalln(`format must be one of "text", "json", or "xml"`)
	}

	// Sleepy?
	{
		var err error
		sleepFor, err = time.ParseDuration(sleepStr)
		if err != nil {
			log.Fatalln("Invalid sleep duration: ", err.Error())
		}
		Debug.Printf("Staggering hosts (sleeping) by %s each\n", sleepFor)
	}

	// cliVars splitting
	if cliVars != "" {
		// CLI
		for c, v := range strings.Split(cliVars, ",") {
			GlobalVars[fmt.Sprintf("VAR%d", c+1)] = v
		}
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

	// If workflow
	//  - ensure the workflow exists
	//  - cache the location of the specified workflow
	//  - ensure we're not executing a must-chain flow
	//  - ensure any required vars exist
	//  - Do The Right Thing
	if workflow != "" {
		wfIndex = conf.WorkflowIndex(workflow)
		if wfIndex < 0 {
			// Does it exist?
			log.Fatalf("Workflow '%s' does not exist in specified configs!\n", workflow)
		} else if conf.Workflows[wfIndex].MustChain {
			// Must we chain it?
			log.Fatalf("Workflow '%s' must be used in a chain!\n", workflow)
		} else {
			// Are all the required CLI vars set?
			req := conf.Workflows[wfIndex].VarsRequired
			for _, r := range req {
				if _, ok := GlobalVars[r]; !ok {
					log.Fatalf("Workflow '%s' requires unset CLI var '%s'!\n", workflow, r)
				}
			}
		}

		if max == 0 {
			// Autoconfig max execs
			max = saneMaxLimitFromWorkflow(conf.Workflows[wfIndex])
		}

		// Init the workflow
		conf.Workflows[wfIndex].Dnf = dnf
		conf.Workflows[wfIndex].Init()

	} else {
		// not a workflow
		wfIndex = -1
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

	// If we're doing a dryrun, this is the end of the line
	if dryrun {
		GlobalVars["dryrun"] = "yup"
	}

	// Status bar!
	// and then the collection phase
	filteredHosts := conf.FilteredHostList(filter, wave, wfIndex)
	filteredHostCount := len(filteredHosts)

	Debug.Printf("FilteredHostCount: %d\n", filteredHostCount)
	bar := pb.New(filteredHostCount)

	if progressBar && logFile != "" {
		Debug.Printf("BAR: Set to %d\n", filteredHostCount)
		bar.Start()
	}

	// We've made it through checks and tests.
	// Let's do this.
	hostList := make(map[string]bool)
	var hostCount time.Duration
	for _, host := range filteredHosts {

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
			User:            configUser,
			Auth:            auths,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		/*
		 * This is where the work is getting accomplished.
		 *   Workflows are configured sets of commands and logics, with sets of returns
		 *   Commands are single directives, with single returns
		 */

		com := Command{Host: host, SSHConfig: config, Sudo: sudo}

		var wait time.Duration
		if sleepFor > 0 && hostCount > 0 {
			wait = hostCount * sleepFor
		}
		hostCount++

		wg.Add(1)
		if workflow != "" {
			// Workflow

			go func() {
				defer wg.Done()
				defer bar.Increment()

				// Sleeeeep
				Debug.Printf("Sleeping for %s\n", wait)
				time.Sleep(wait)

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
				defer wg.Done()
				defer bar.Increment()

				// Sleeeeep
				Debug.Printf("Sleeping for %s\n", wait)
				time.Sleep(wait)

				sem.Lock()
				defer sem.Unlock()

				commandResults <- com.Exec()
			}()
		}
	}

	/*
	 * Post-run
	 *
	 */

	// We wait for all the goros to finish up
	for i := 0; i < len(hostList); i++ {
		if workflow != "" {
			// Workflow
			select {
			case res := <-wfResults:
				hostList[res.HostObj.Name] = true // returned is good enough for this

				if !res.Completed {
					Error.Printf("Workflow %s did not fully complete\n", res.Name)
				}

				if !quiet {
					// Process all of the enclosed CommandReturns

					for _, c := range res.CommandReturns {
						if c.Quiet {
							continue
						}
						switch format {
						case "xml":
							Log.Println(string(c.ToXML()))
						case "json":
							Log.Println(string(c.ToJSON(false)))
						case "text":
							fallthrough
						default:
							Log.Println(c.ToText())
						}
					}

				}
			case <-time.After(time.Duration(timeout) * time.Second):
				var badHosts []string
				for h, v := range hostList {
					if !v {
						badHosts = append(badHosts, h)
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

				if !quiet && !res.Quiet {
					switch format {
					case "xml":
						Log.Println(string(res.ToXML()))
					case "json":
						Log.Println(string(res.ToJSON(false)))
					case "text":
						fallthrough
					default:
						Log.Println(res.ToText())
					}
				}
			case <-time.After(time.Duration(timeout) * time.Second):
				var badHosts []string
				for h, v := range hostList {
					if !v {
						badHosts = append(badHosts, h)
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
	wg.Wait()
	if progressBar && logFile != "" {
		bar.Finish()
	}
}
