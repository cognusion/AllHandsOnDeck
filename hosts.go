package main

import (
	"log"
	"sort"
	"strings"
)

type Host struct {
	Port    int
	Address string
	Name    string
	Arch    string
	AltUser string
	Tags    []string
}

func (h *Host) SortTags() {
	if sort.StringsAreSorted(h.Tags) == false {
		sort.Strings(h.Tags)
	}
}

func (h *Host) SearchTags(tag string) bool {

	for _,t := range h.Tags {
		if t == tag {
			//log.Printf("\tFound %s!\n",tag)
			return true
		}
	}
	//log.Printf("\tDidn't find %s\n",tag)
	return false
}

/*
	Tags == "dev" and Tags == "httpd" or Tags == "haproxy" or Tags == "tomcat" and Tags == "daisy"
	
	1 Tags == "dev" 
	3 Tags == "httpd" or Tags == "haproxy" or Tags == "tomcat"
	2 Tags == "daisy"

*/
func (h *Host) If(cond string) bool {
	
	//log.Printf("COND: %s\n",cond)
	
	if strings.Contains(cond," and ") {
		ands := strings.Split(cond," and ")
		for _,a := range ands {
			//log.Printf("\tAND: %s\n",a)
			ret := h.If(a)
			if ret == false {
				return false
			}
		}
		return true
	} else if strings.Contains(cond, " or ") {
		ors := strings.Split(cond," or ")
		for _,o := range ors {
			//log.Printf("\tOR: %s\n",o)
			ret := h.If(o)
			if ret == true {
				return true
			}
		}
		return false
	} else {
		// Single statement
		parts := strings.Split(cond," ")
		
		//log.Printf("\tDoes %s %s %s?\n",parts[0],parts[1],parts[2])
		if parts[1] == "==" {
			// Equality
			if parts[0] == "Tags" {
				if h.SearchTags(parts[2]) {
					return true
				} else {
					return false
				}
			}
		} else if parts[1] == "!=" {
			// Inequality
			if parts[0] == "Tags" {
				if h.SearchTags(parts[2]) {
					return false
				} else {
					return true
				}
			}
		}
	}
	
	log.Printf("We should never get here... %s\n",cond)
	return false
}
