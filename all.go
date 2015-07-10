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
	"strings"
	"time"
)

const (
	username = "M"
	server   = "staging.sk.com:22"
)

type Config struct {
	Hosts []Host
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
		filter  string
		debug      bool

		conf  Config
		auths []ssh.AuthMethod
	)

	currentUser, _ := user.Current()
	results := make(chan CommandReturn, 10)

	flag.BoolVar(&sshAgent, "sshagent", false, "Connect and use SSH-Agent vs. user key")
	flag.StringVar(&sshKey, "sshkey", currentUser.HomeDir+"/.ssh/id_rsa", "If not using the SSH-Agent, where to grab the key")
	flag.BoolVar(&debug, "debug", false, "Enable Debug output")
	flag.StringVar(&configFile, "config", "", "Config file location to read and run from")
	flag.StringVar(&userName, "user", currentUser.Username, "User to run as")
	flag.IntVar(&timeout, "timeout", 5, "Seconds before command times out")
	flag.BoolVar(&sudo, "sudo", false, "Whether to run commands via sudo")
	flag.StringVar(&cmd,"cmd","","Command to run")
	flag.StringVar(&filter,"filter","","Boolean expression to positively filter on Tags")
	flag.Parse()

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
	
	if cmd == "" {
		log.Fatalln("cmd must be set!")
	}
	
	if sudo {
		cmd = "sudo " + cmd
	}
	
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
		// Use a key

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
	
			
		if filter != "" && host.If(filter) == false {
			hostCount--
			continue
		}
		
		log.Printf("Host: %s\n",host.Name)
		
		configUser := username
		
		if host.AltUser != "" {
			configUser = host.AltUser
		}
	
		config := &ssh.ClientConfig{
			User: configUser,
			Auth: auths,
		}
		
		go func(host Host) {
			results <- executeCommand(cmd, host, config, sudo)
		}(host)
	}

	for i := 0; i < hostCount; i++ {
		select {
		case res := <-results:
			if res.Error != nil {
				log.Println(res.Error)
			} else if strings.Contains(cmd, "needs-restarting") {
				plist := plistToInits(res.StdoutStrings())
				log.Printf("%s:\n%v\n",res.Hostname,plist)	
			} else {
				log.Printf("%s:\nxxxxxxxx\n%s\nyyyyyyyyy\n%s\nzzzzzzzzz\n", res.Hostname, res.StdoutString(), res.StderrString())
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			log.Println("Timed out!")
			return
		}
	}
}
