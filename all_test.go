package main

import (
	"testing"
)

func TestAll_LoadConfigs(t *testing.T) {

	conf := loadConfigs("testconfigs/")
	if len(conf.Hosts) < 1 {
		t.Error("Expected at least one Host, got 0")
	}
	if len(conf.Workflows) < 1 {
		t.Error("Expected at least one Workflow, got 0")
	}
	if len(conf.Miscs) < 1 {
		t.Error("Expected at least one Misc, got 0")
	}
}

func TestAll_ConfigMerge(t *testing.T) {

	// empties
	var c1 Config
	var c2 Config
	var c3 Config

	c4 := loadConfigFile("testconfigs/testdevhosts.json", c1)
	num1 := len(c4.Hosts)
	c5 := loadConfigFile("testconfigs/testprodhosts.json", c2)
	num2 := len(c5.Hosts)
	c6 := loadConfigFile("testconfigs/testmoarhosts.json", c3)
	num3 := len(c6.Hosts)
	merge1 := c4
	merge1.Merge(c5)

	if len(merge1.Hosts) != num1+num2 {
		t.Errorf("Expected %d, got %d\n", num1+num2, len(merge1.Hosts))
	}

	c6.Merge(merge1)
	if len(c6.Hosts) != num1+num2+num3 {
		t.Errorf("Expected %d, got %d\n", num1+num2+num3, len(c6.Hosts))
	}
}

func TestAll_WorkflowIndex(t *testing.T) {
	var conf Config
	conf = loadConfigFile("testconfigs/testflows.json", conf)

	if len(conf.Workflows) != 5 {
		t.Error("Expected 5 workflows, got ", len(conf.Workflows))
	}

	if conf.WorkflowIndex("restart-tomcat") < 0 {
		t.Error("Unexpectedly not finding workflow 'restart-tomcat'")
	}

	if conf.WorkflowIndex("NOPE") >= 0 {
		t.Error("Unexpectedly found workflow 'NOPE'")
	}
}

func TestAll_WorkflowChain(t *testing.T) {
	var conf Config
	conf = loadConfigFile("testconfigs/testflows.json", conf)

	// TODO: test chaining workflows
}

func TestAll_WorkflowMustChain(t *testing.T) {
	var conf Config
	conf = loadConfigFile("testconfigs/testflows.json", conf)

	// TODO: test chaining workflows with and without MustChain
}
