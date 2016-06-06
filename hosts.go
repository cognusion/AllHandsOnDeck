// +build go1.4

package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Host is a structure to capture properties of individual systems
type Host struct {
	Address string
	Arch    string
	Loc     string
	Wave    int
	Name    string
	Offline bool
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

	//Debug.Printf("COND: %s\n",cond)
	if cond == "" { return true }

	// Standardize the ands and ors
	rAnd := regexp.MustCompile(`(?i) and `)
	rOr := regexp.MustCompile(`(?i) or `)
	cond = rAnd.ReplaceAllString(cond, " && ")
	cond = rOr.ReplaceAllString(cond, " || ")

	// Parse
	if strings.Contains(cond, " && ") {
		return h.And(strings.Split(cond, " && "))

	} else if strings.Contains(cond, " || ") {
		return h.Or(strings.Split(cond, " || "))

	} else {
		// Single statement
		parts := strings.Fields(cond)

		//Debug.Printf("\tDoes %s %s %s?\n",parts[0],parts[1],parts[2])

		// Case/swtich to check each of the fields
		found := false
		switch parts[0] {
		case "Tags":
			found = h.SearchTags(parts[2])
		case "Port":
			fport, _ := strconv.Atoi(parts[2])
			if h.Port != 0 {
				found = h.Port == fport
			} else {
				// we started allowing port to be skipped
				found = 22 == fport
			}
		case "Wave":
			fwave, _ := strconv.Atoi(parts[2])
			if h.Wave != 0 {
				found = h.Wave == fwave
			}
		case "Address":
			found = h.Address == parts[2]
		case "Loc":
			found = h.Loc == parts[2]
		case "Name":
			found = h.Name == parts[2]
		case "Arch":
			found = h.Arch == parts[2]
		case "User":
			// caveat: We don't have access to the CLI-specified user,
			// so this only matches a host-specified user
			found = h.User == parts[2]
		default:
			// Hmmm...
			Debug.Printf("Conditional name '%s' does not exist!\n", parts[0])
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

func (h *Host) And(conds []string) bool {
	for _, a := range conds {
		ret := h.If(a)
		if ret == false {
			return false
		}
	}
	return true
}

func (h *Host) Or(conds []string) bool {
	for _, o := range conds {
		//Debug.Printf("\tOR: %s\n",o)
		ret := h.If(o)
		if ret == true {
			return true
		}
	}
	return false
}
