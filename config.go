package main

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