// +build go1.4

package main

import (
	"sort"
	"strconv"
	"strings"
)

// Host is a structure to capture properties of individual systems
type Host struct {
	Address string
	Arch    string
	Loc     string
	Name    string
	Port    int
	Tags    []string
	User    string
}

// SortTags sorts the Host's Tags array in alphanumeric order
func (h *Host) SortTags() {
	if sort.StringsAreSorted(h.Tags) == false {
		sort.Strings(h.Tags)
	}
}

// SearchTags iterates over the Tags array and return true/false if the requested tag is found
func (h *Host) SearchTags(tag string) bool {
	for _, t := range h.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

/*
	If takes a condition list ("filter") and applies it to the Host.
	Tags == dev and Tags == httpd or Tags == haproxy or Tags == tomcat and Tags == daisy

	Tags == dev
	&&
	Tags == httpd or Tags == haproxy or Tags == tomcat
	&&
	Tags == daisy
*/
func (h *Host) If(cond string) bool {

	//debugOut.Printf("COND: %s\n",cond)

	if strings.Contains(cond, " and ") {
		ands := strings.Split(cond, " and ")
		for _, a := range ands {
			//debugOut.Printf("\tAND: %s\n",a)
			ret := h.If(a)
			if ret == false {
				return false
			}
		}
		return true
	} else if strings.Contains(cond, " or ") {
		ors := strings.Split(cond, " or ")
		for _, o := range ors {
			//debugOut.Printf("\tOR: %s\n",o)
			ret := h.If(o)
			if ret == true {
				return true
			}
		}
		return false
	} else {
		// Single statement
		parts := strings.Split(cond, " ")

		//debugOut.Printf("\tDoes %s %s %s?\n",parts[0],parts[1],parts[2])

		// Case/swtich to check each of the fields
		found := false
		if parts[0] == "Tags" {
			found = h.SearchTags(parts[2])
		} else if parts[0] == "Port" {
			fport, _ := strconv.Atoi(parts[2])
			if h.Port != 0 {
				found = h.Port == fport
			} else {
				// we started allow port to be skipped
				found = 22 == fport
			}
		} else if parts[0] == "Address" {
			found = h.Address == parts[2]
		} else if parts[0] == "Loc" {
			found = h.Loc == parts[2]
		} else if parts[0] == "Name" {
			found = h.Name == parts[2]
		} else if parts[0] == "Arch" {
			found = h.Arch == parts[2]
		} else if parts[0] == "User" {
			// caveat: We don't have access to the CLI-specified user,
			// so this only matches a host-specified user
			found = h.User == parts[2]
		} else {
			// Hmmm...
			debugOut.Printf("Conditional name '%s' does not exist!\n", parts[0])
			return false
		}

		// Case/switch to check each operator
		if parts[1] == "==" && found {
			return true
		} else if parts[1] == "!=" && found == false {
			return true
		} else {
			return false
		}
	}
}
