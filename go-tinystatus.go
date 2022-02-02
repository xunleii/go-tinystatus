package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// defines go-tinystatus logger
var log = zerolog.New(os.Stdout).
	With().Timestamp().
	Logger()

func main() {
	checkFile, _ := os.Open("checks.csv")
	defer checkFile.Close()

	records, err := extractRecordsFromCSV(checkFile)
	if err != nil {
		panic(err)
	}

	status := StatusList{}
	for _, record := range records {
		status = append(status, Probes[record.CType](record))
	}

	buff := bytes.NewBufferString("")
	err = templatedHtml.ExecuteTemplate(buff, "tinystatus", status)
	if err != nil {
		panic(err)
	}
	fmt.Println(buff.String())
}

type (
	// Status is a union of a record and its given scan result.
	Status struct {
		*RecordStatus
		ProbeResult
	}
	// StatusList is a list of Status with some sugar to use them
	// more easily with go templates.
	StatusList []Status
)

func (l StatusList) Len() int      { return len(l) }
func (l StatusList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l StatusList) Less(i, j int) bool {
	if cmp := strings.Compare(l[i].Category, l[j].Category); cmp != 0 {
		return cmp < 0
	}
	return l[i].Succeed() == true != l[j].Succeed()
}

// Categories returns all status organized by category
func (l StatusList) Categories() map[string][]Status {
	sort.Sort(l)
	categories := map[string][]Status{}
	for _, status := range l {
		categories[status.Category] = append(categories[status.Category], status)
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

// LastCheck returns the current date (helpers for go template).
func (l StatusList) LastCheck() time.Time { return time.Now() }

// Succeed returns true if the scan didn't find any error.
func (s Status) Succeed() bool { return s.ProbeResult == nil }

type (
	// RecordStatus represents a check that go-tinystatus should check
	RecordStatus struct {
		Category, Name             string
		CType, Target, Expectation string
	}
	// ProbeResult is the result of a probe scan on a record
	ProbeResult error
)

// extractRecordsFromCSV read the CSV file and extract all record.
func extractRecordsFromCSV(file *os.File) ([]RecordStatus, error) {
	var records []RecordStatus

	scanner, line := bufio.NewScanner(file), 0
	for scanner.Scan() {
		line++
		record := strings.Split(scanner.Text(), ",")

		if len(record) < 4 {
			log.Error().Msgf("invalid CSV row %d: wrong number of fields", line)
			continue
		}

		ctype := strings.TrimSpace(record[0])
		if _, exists := Probes[ctype]; !exists {
			log.Warn().Msgf("unknown probe '%s'", ctype)
			continue
		}

		rs := RecordStatus{
			CType:       ctype,
			Category:    "Services",
			Expectation: strings.TrimSpace(record[1]),
			Name:        strings.TrimSpace(record[2]),
			Target:      strings.TrimSpace(record[3]),
		}
		if len(record) >= 5 {
			rs.Category = strings.TrimSpace(record[4])
		}

		records = append(records, rs)
	}

	return records, scanner.Err()
}
