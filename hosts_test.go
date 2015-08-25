package main

import (
	"sort"
	"testing"
)

var host1 *Host = &Host{
	Address: "1.2.3.4",
	Arch:    "x86_64",
	Loc:     "east",
	Name:    "testhost1",
	Wave:    1,
	Offline: false,
	Port:    22,
	Tags:    []string{"tag3", "tag2", "tag1", "tag4"},
	User:    "auser",
}

// Tags == dev and Tags == httpd or Tags == haproxy or Tags == tomcat and Tags == daisy
var host2 *Host = &Host{
	Tags: []string{"dev", "httpd", "tomcat", "daisy"},
}

func TestHost_Wave(t *testing.T) {
	if host1.If("Wave != 1") {
		t.Error("Wave not evaluated properly")
	}
}

func TestHost_TagSort(t *testing.T) {
	if sort.StringsAreSorted(host1.Tags) == true {
		t.Error("Tags should not be sorted already, but are!")
	}

	host1.SortTags()
	if sort.StringsAreSorted(host1.Tags) == false {
		t.Error("Tags should be sorted, but aren't: ", host1.Tags)
	}
}

func TestHost_TagSearch(t *testing.T) {
	if host1.SearchTags("tag2") == false {
		t.Error("Expecting to find tag2, but didn't: ", host1.Tags)
	}

	if host1.SearchTags("NOPE") {
		t.Error("Not expecting to find NOPE, but did: ", host1.Tags)
	}
}

func TestHost_SimpleFilters(t *testing.T) {
	ap := host1.If("Address == 1.2.3.4")
	an := host1.If("Address != 1.2.3.4")
	if ap == false {
		t.Error("Address == 1.2.3.4 should be true, but false!")
	}
	if an {
		t.Error("Address != 1.2.3.4 should be false, but true!")
	}

	pp := host1.If("Port == 22")
	pn := host1.If("Port != 22")
	if pp == false {
		t.Error("Port == 22 should be true, but false!")
	}
	if pn {
		t.Error("Port != 22 should be false, but true!")
	}

	tp := host1.If("Tags == tag1")
	tn := host1.If("Tags != tag1")
	if tp == false {
		t.Error("Tags == tag1 should be true, but false!")
	}
	if tn {
		t.Error("Tags != tag1 should be false, but true!")
	}

	andp := host1.If("Tags == tag1 and Address == 1.2.3.4")
	andn := host1.If("Tags == NOPE and Address == 1.2.3.4")
	if andp == false {
		t.Error("Tags == tag1 and Address == 1.2.3.4 should be true, but false!")
	}
	if andn {
		t.Error("Tags == NOPE and Address == 1.2.3.4 should be false, but true!")
	}

	orp := host1.If("Tags == NOPE or Address == 1.2.3.4")
	orn := host1.If("Tags == NOPE or Address == NOPE")
	if orp == false {
		t.Error("Tags == NOPE or Address == 1.2.3.4 should be true, but false!")
	}
	if orn {
		t.Error("Tags == NOPE or Address == NOPE should be false, but true!")
	}
}

func TestHost2_ComplexFilter(t *testing.T) {
	f := "Tags == dev and Tags == httpd or Tags == haproxy or Tags == tomcat and Tags == daisy"
	fn := "Tags == dev and Tags == httpd or Tags == haproxy or Tags == tomcat and Tags == dipsy"
	fo := "Tags == dev and Tags == NOPE or Tags == NOT or Tags == HRM and Tags == daisy"

	if host2.If(f) == false {
		t.Errorf("Expecting true for '%s' from %s\n", f, host2.Tags)
	}
	if host2.If(fn) {
		t.Errorf("Expecting false for '%s' from %s\n", fn, host2.Tags)
	}
	if host2.If(fo) {
		t.Errorf("Expecting false for '%s' from %s\n", fo, host2.Tags)
	}
}
