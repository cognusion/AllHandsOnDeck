package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Config is a toplevel struct to house arrays of Hosts, Workflows, and Miscs
type Config struct {
	Hosts     []Host
	Workflows []Workflow
	Miscs     []Misc
}

// Adds a Host to Hosts
func (c *Config) AddHost(h Host) {
	c.Hosts = append(c.Hosts, h)
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

// Given a filter, count the matching hosts
func (c *Config) FilteredHostList(filter string, wave, workflowIndex int) (hosts []Host) {

	for _, host := range c.Hosts {

		if host.Offline == true {
			// Check to see if the host is offline
			continue
		} else if wave > 0 && host.Wave != wave {
			// Check to see if we're using waves, and if this is in it
			continue
		} else if filter != "" && host.If(filter) == false {
			// Check to see if the this host matches our filter
			continue
		} else if workflowIndex >= 0 && host.If(c.Workflows[workflowIndex].Filter) == false {
			// Check to see if we're using workflows, and if this is in it
			continue
		}

		// POST: We're interested in this host
		hosts = append(hosts, host)
	}

	return
}

// Return a formatted JSON string representation of the config
func dumpConfigs(conf Config) string {
	j, _ := json.MarshalIndent(conf, "", "\t")
	return string(j)
}

// Given a directory, load all the configs
func loadConfigs(srcDir string) Config {
	var conf Config
	Debug.Printf("Looking for configs in '%s'\n", srcDir)
	for _, f := range readDirectory(srcDir, "*.json") {
		Debug.Printf("\tReading config '%s'\n", f)
		conf = loadConfigFile(f, conf)
	}
	return conf
}

// Load the given config file into the specified config
func loadConfigFile(filePath string, conf Config) Config {

	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading config file '%s': %s\n", filePath, err)
	}

	var newConf Config

	err = json.Unmarshal(buf, &newConf)
	if err != nil {
		log.Fatalf("Error parsing JSON in config file '%s': %s\n", filePath, err)
	}

	conf.Merge(newConf)
	return conf
}
