package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// StatusPage contains all data required to generate the status page.
type StatusPage struct {
	statusChecksPath       string
	statusChecks           StatusCheckList
	lastStatusChecksUpdate time.Time

	incidentsPath       string
	incidents           []string
	lastIncidentsUpdate time.Time
}

// RenderHTML runs all configured checks in parallel, fetch all last incidents
// and generates the HTML page.
func (p StatusPage) RenderHTML(ctx context.Context) (string, error) {
	start := time.Now()

	checkList, err := p.StatusChecks(ctx)
	if err != nil {
		return "", err
	}
	statuses := checkList.RunAll(ctx)

	incidents, err := p.Incidents(ctx)
	if err != nil {
		return "", err
	}

	data := map[string]interface{}{
		"Statuses":  statuses,
		"Incidents": incidents,

		"LastCheck": start,
		"Elapsed":   time.Since(start),
	}

	buff := bytes.NewBufferString("")
	err = templatedHtml.ExecuteTemplate(buff, "tinystatus", data)
	return buff.String(), err
}

// StatusChecks reads the CSV file containing the list of checks, parses it and
// returns the list of StatusCheck.
func (p *StatusPage) StatusChecks(ctx context.Context) (StatusCheckList, error) {
	logger := zerolog.Ctx(ctx)

	// NOTE: check if the file has changed since last time we read it and
	// 		 update cached data otherwise
	{
		stats, err := os.Stat(p.statusChecksPath)
		if err != nil {
			return p.statusChecks, err
		}

		if !stats.ModTime().After(p.lastStatusChecksUpdate) {
			logger.Debug().Msgf("'%s' not modified since last check", p.statusChecksPath)
			return p.statusChecks, nil
		}

		p.lastStatusChecksUpdate = stats.ModTime()
	}

	file, err := os.Open(p.statusChecksPath)
	if err != nil {
		return p.statusChecks, err
	}

	var checkList StatusCheckList

	scanner := bufio.NewScanner(file)
	for nline := 0; scanner.Scan(); nline++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			// NOTE: ignore empty or commented lines
			continue
		}

		check, err := NewStatusCheck(line)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to parse CSV line %d", nline)
			continue
		}

		checkList = append(checkList, check)
	}

	if err := scanner.Err(); err != nil {
		return p.statusChecks, fmt.Errorf("failed to extract CSV rows from '%s': %w", p.statusChecksPath, err)
	}
	_ = file.Close()

	p.statusChecks = checkList
	return p.statusChecks, nil
}

// Incidents reads the CSV file containing the list of incidents and returns
// the list of incidents.
func (p *StatusPage) Incidents(ctx context.Context) ([]string, error) {
	logger := zerolog.Ctx(ctx)

	// NOTE: check if the file has changed since last time we read it and
	// 		 update cached data otherwise
	{
		stats, err := os.Stat(p.incidentsPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return p.incidents, err
		}

		if !stats.ModTime().After(p.lastIncidentsUpdate) {
			logger.Debug().Msgf("'%s' not modified since last check", p.incidentsPath)
			return p.incidents, nil
		}

		p.lastIncidentsUpdate = stats.ModTime()
	}

	var incidents []string
	file, err := os.Open(p.incidentsPath)
	if err != nil {
		return p.incidents, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		incidents = append(incidents, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return p.incidents, fmt.Errorf("failed to extract lines from '%s': %w", p.statusChecksPath, err)
	}
	_ = file.Close()

	p.incidents = incidents
	return incidents, nil
}
