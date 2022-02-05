package main

import (
	"sort"
	"strings"
)

type (
	// Status is a union of a record and its given scan result.
	Status struct {
		*RecordStatus
		ProbeResult
	}
	// StatusList is a list of Status with some sugar to use them
	// more easily with go templates.
	StatusList []Status

	// RecordStatus represents a check that go-tinystatus should check
	RecordStatus struct {
		Category, Name             string
		CType, Target, Expectation string
	}
	// ProbeResult is the result of a probe scan on a record
	ProbeResult error
)

func (l StatusList) Len() int      { return len(l) }
func (l StatusList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l StatusList) Less(i, j int) bool {
	if cmp := strings.Compare(l[i].Category, l[j].Category); cmp != 0 {
		return cmp < 0
	}
	return strings.Compare(l[i].Name, l[j].Name) < 0
}

// Categories returns all status organized by category
func (l StatusList) Categories() map[string][]Status {
	sort.Sort(l)
	categories := map[string][]Status{}
	for _, status := range l {
		categories[status.Category] = append(categories[status.Category], status)
	}

	// NOTE: to keep retro-compatibility with tinystatus, if there is only 1 category
	//  	 named `Uncategorized`, it should be renamed `Services`.
	if len(categories) == 1 && categories["Uncategorized"] != nil {
		categories["Services"] = categories["Uncategorized"]
		delete(categories, "Uncategorized")
	}

	return categories
}

// NumberOutages returns the number of outages found.
func (l StatusList) NumberOutages() int {
	nb := 0
	for _, status := range l {
		if !status.Succeed() {
			nb++
		}
	}
	return nb
}

// Succeed returns true if the scan didn't find any error.
func (s Status) Succeed() bool { return s.ProbeResult == nil }
