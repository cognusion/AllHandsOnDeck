//go:build go1.4

package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Host is a structure to capture properties of individual systems
type Host struct {
	Address            string
	Arch               string
	Loc                string
	Wave               int
	Name               string
	Offline            bool
	Port               int
	Tags               []string
	User               string
	DontUpdatePackages string
}

// SortTags sorts the Host's Tags array in alphanumeric order
func (h *Host) SortTags() {
	if !sort.StringsAreSorted(h.Tags) {
		sort.Strings(h.Tags)
	}
}

// SearchTags iterates over the Tags array and return true/false if the requested tag is found
func (h *Host) SearchTags(tag string, fuzzy bool) bool {
	for _, t := range h.Tags {
		if !fuzzy && t == tag {
			return true
		} else if fuzzy && strings.Contains(t, tag) {
			return true
		}
	}
	return false
}

/*If takes a condition list ("filter") and applies it to the Host.
Tags == dev and Tags == httpd or Tags == haproxy or Tags == tomcat and Tags == daisy

Tags == dev
&&
Tags == httpd or Tags == haproxy or Tags == tomcat
&&
Tags == daisy */
func (h *Host) If(cond string) bool {

	//Debug.Printf("COND: %s\n",cond)
	if cond == "" {
		return true
	}

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

		if len(parts) != 3 {
			Error.Printf("Statement syntax invalid! '%s'\n", cond)
			return false
		}

		field, op, val := parts[0], parts[1], parts[2]
		fuzzy := false

		// Check for operator existence
		switch op {
		case "==":
		case "!=":
		case "~!":
			fuzzy = true
		case "~=":
			fuzzy = true
		default:
			Error.Printf("Operator '%s' does not exist!\n", op)
			return false
		}

		// Case/swtich to check each of the fields
		found := false
		switch field {
		case "Tags":
			found = h.SearchTags(val, fuzzy)
		case "Port":
			fport, _ := strconv.Atoi(val)
			if h.Port != 0 {
				found = h.Port == fport
			} else {
				// we started allowing port to be skipped
				found = fport == 22
			}
		case "Wave":
			fwave, _ := strconv.Atoi(val)
			if h.Wave != 0 {
				found = h.Wave == fwave
			}
		case "Address":
			if fuzzy {
				found = strings.Contains(h.Address, val)
			} else {
				found = h.Address == val
			}
		case "Loc":
			if fuzzy {
				found = strings.Contains(h.Loc, val)
			} else {
				found = h.Loc == val
			}
		case "Name":
			if fuzzy {
				found = strings.Contains(h.Name, val)
			} else {
				found = h.Name == val
			}
		case "Arch":
			if fuzzy {
				found = strings.Contains(h.Arch, val)
			} else {
				found = h.Arch == val
			}
		case "User":
			// caveat: We don't have access to the CLI-specified user,
			// so this only matches a host-specified user
			if fuzzy {
				found = strings.Contains(h.User, val)
			} else {
				found = h.User == val
			}
		default:
			// Hmmm...
			Error.Printf("Conditional name '%s' does not exist!\n", field)
			return false
		}

		// Case/switch to check each operator
		if found && (op == "==" || op == "~=") {
			return true
		} else if !found && (op == "!=" || op == "~!") {
			return true
		} else {
			// Fail safe
			return false
		}
	}
}

// And returns true if all of the conditions are true
func (h *Host) And(conds []string) bool {
	for _, a := range conds {
		ret := h.If(a)
		if !ret {
			return false
		}
	}
	return true
}

// Or returns true if any of the conditions are true
func (h *Host) Or(conds []string) bool {
	for _, o := range conds {
		//Debug.Printf("\tOR: %s\n",o)
		ret := h.If(o)
		if ret {
			return true
		}
	}
	return false
}
