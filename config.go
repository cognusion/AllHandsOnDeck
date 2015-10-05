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

func dumpConfigs(conf Config) string {
	j, _ := json.MarshalIndent(conf, "", "\t")
	return string(j)
}

func loadConfigs(srcDir string) Config {
	var conf Config
	Debug.Printf("Looking for configs in '%s'\n", srcDir)
	for _, f := range readDirectory(srcDir) {
		Debug.Printf("\tReading config '%s'\n", f)
		conf = loadConfigFile(f, conf)
	}
	return conf
}

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
