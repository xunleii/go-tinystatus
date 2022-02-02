package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/integrii/flaggy"
	"github.com/rs/zerolog"
)

var log = zerolog.New(os.Stderr).
	With().Timestamp().Caller().
	Logger()

func main() {
	// cli: go-tinystatus [--daemon <host>:<port>] [--check <row>] <check.csv> <incidents.txt>
	var checkPath string = "checks.csv"
	var incidentsPath string = "incidents.txt"

	flaggy.AddPositionalValue(&checkPath, checkPath, 1, false, "File containing all checks, formatted in CSV")
	flaggy.AddPositionalValue(&incidentsPath, incidentsPath, 2, false, "File containing all incidents to be displayed")
	flaggy.Parse()

	var records []RecordStatus
	checkFile, err := os.Open(checkPath)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	records, err = extractRecordsFromCSV(checkFile)
	_ = checkFile.Close()
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to extract CSV rows from '%s'", checkPath)
	}

	var incidents []string
	if _, err := os.Stat(incidentsPath); !errors.Is(err, os.ErrNotExist) {
		incidentsFile, err := os.Open(incidentsPath)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		incidents, err = extractIncidentsFromTxt(incidentsFile)
		_ = incidentsFile.Close()
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to extract incidents from '%s'", checkPath)
		}
	}

	status := StatusList{}
	for _, record := range records {
		status = append(status, Probes[record.CType](record))
	}

	data := map[string]interface{}{
		"Status":    status,
		"Incidents": incidents,
		"LastCheck": time.Now(),
	}

	buff := bytes.NewBufferString("")
	err = templatedHtml.ExecuteTemplate(buff, "tinystatus", data)
	if err != nil {
		panic(err)
	}
	fmt.Println(buff.String())
}

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

// extractIncidentsFromTxt return all line in an array off string
func extractIncidentsFromTxt(file *os.File) ([]string, error) {
	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	return lines, scanner.Err()
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
