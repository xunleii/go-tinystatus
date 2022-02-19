package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/integrii/flaggy"
	"github.com/rs/zerolog"
)

func main() {
	page := StatusPage{
		CheckPath:     "checks.csv",
		IncidentsPath: "incidents.txt",
		PageTitle:     "tinystatus",
	}

	var (
		daemonize = false
		addr      = ":8080"
		interval  = 15 * time.Second
		logLevel  = zerolog.LevelInfoValue
	)
	flaggy.DefaultParser.DisableShowVersionWithVersion()

	flaggy.AddPositionalValue(&page.CheckPath, page.CheckPath, 1, false, "File containing all checks, formatted in CSV")
	flaggy.AddPositionalValue(&page.IncidentsPath, page.IncidentsPath, 2, false, "File containing all incidents to be displayed")

	flaggy.String(&page.PageTitle, "", "title", "Title of the status page.")
	flaggy.Duration(&timeout, "", "timeout", "Maximum time to wait a probe before aborting.")
	flaggy.Bool(&daemonize, "", "daemon", "Start go-tinystatus as daemon with an embedded web server.")
	flaggy.String(&addr, "", "addr", "Address on which the daemon will be listening.")
	flaggy.Duration(&interval, "", "interval", "Interval between two page rendering.")
	flaggy.String(&logLevel, "", "level", "Log verbosity.")

	flaggy.Parse()

	lvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stderr).
		Level(lvl).
		With().Timestamp().
		Logger()
	page.ctx = logger.WithContext(context.Background())

	html, err := page.RenderHTML()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to render the status page")
	}

	if !daemonize {
		fmt.Print(html)
		return
	}

	_, done := context.WithCancel(page.ctx)
	rwx, ticker := sync.RWMutex{}, time.NewTicker(interval)
	go func() {
		for range ticker.C {
			newHTML, rendErr := page.RenderHTML()
			if rendErr != nil {
				logger.Error().Err(rendErr).Msg("failed to render the status page")
			}

			rwx.Lock()
			html = newHTML
			rwx.Unlock()
		}
	}()

	logger.Info().Msgf("start go-tinystatus listening on '%s'", addr)
	err = http.ListenAndServe(addr, http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
		rwx.RLock()
		defer rwx.RUnlock()
		_, _ = wr.Write([]byte(html))
	}))
	done() // NOTE: cancel the current context to clean current connections

	if err != nil {
		logger.Fatal().Err(err).Send()
	}
}

// StatusPage contains all data required to generate the status page.
type StatusPage struct {
	CheckPath     string
	IncidentsPath string
	PageTitle     string

	records           []RecordStatus
	lastRecordsUpdate time.Time

	start time.Time
	ctx   context.Context
}

// RenderHTML runs all checks in parallel and generate the HTML page.
func (p StatusPage) RenderHTML() (string, error) {
	p.start = time.Now()

	buff := bytes.NewBufferString("")
	err := templatedHtml.ExecuteTemplate(buff, "tinystatus", p)
	return buff.String(), err
}

// LastCheck is used during HTML rendering to know when the page has
// been generated.
func (p StatusPage) LastCheck() time.Time { return time.Now() }

// Elapsed returns how much time it took to make all checks.
func (p StatusPage) Elapsed() time.Duration { return time.Since(p.start) }

// Records reads the CSV file containing the list of checks, parses it and
// returns the list of RecordStatus representing a check.
func (p *StatusPage) Records() ([]RecordStatus, error) {
	logger := zerolog.Ctx(p.ctx).
		With().Str("component", "StatusPage.Record").
		Logger()

	stats, err := os.Stat(p.CheckPath)
	if err != nil {
		return p.records, err
	}

	if !stats.ModTime().After(p.lastRecordsUpdate) {
		logger.Debug().Msgf("'%s' not modified since last check", p.CheckPath)
		return p.records, nil
	}

	file, err := os.Open(p.CheckPath)
	if err != nil {
		return p.records, err
	}

	var records []RecordStatus

	scanner := bufio.NewScanner(file)
	for nline := 0; scanner.Scan(); nline++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			// NOTE: ignore empty or commented lines
			continue
		}

		record := strings.Split(line, ",")
		if len(record) < 4 {
			logger.Error().Msgf("invalid CSV row %d: wrong number of fields", nline)
			continue
		}

		ctype := strings.ToLower(strings.TrimSpace(record[0]))
		if _, exists := Probes[ctype]; !exists {
			logger.Warn().Msgf("unknown probe '%s'", ctype)
			continue
		}

		rs := RecordStatus{
			CType:    ctype,
			Category: "Uncategorized", Name: strings.TrimSpace(record[2]),
			Target: strings.TrimSpace(record[3]), Expectation: strings.TrimSpace(record[1]),
		}
		if len(record) >= 5 && strings.TrimSpace(record[4]) != "" {
			rs.Category = strings.TrimSpace(record[4])
		}

		records = append(records, rs)
	}

	if err := scanner.Err(); err != nil {
		return p.records, fmt.Errorf("failed to extract CSV rows from '%s': %w", p.CheckPath, err)
	}
	_ = file.Close()

	p.lastRecordsUpdate = stats.ModTime()
	p.records = records
	return p.records, nil
}

// Status runs all checks in parallel and returns the list of results.
func (p StatusPage) Status() StatusList {
	logger := zerolog.Ctx(p.ctx).
		With().Str("component", "StatusPage.Status").
		Logger()

	records, err := p.Records()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get the list of checks")
		// NOTE: we don't return an error here because we want to continue
		// 		 with old records
	}

	// NOTE: limit the number of check to 32 requests in parallel
	ratelimit, wg, mx := make(chan struct{}, 32), sync.WaitGroup{}, sync.Mutex{}
	status := StatusList{}
	for _, record := range records {
		wg.Add(1)
		ratelimit <- struct{}{}

		go func(record RecordStatus) {
			defer wg.Done()
			defer func() { <-ratelimit }()
			result := Probes[record.CType](p.ctx, &record)

			mx.Lock()
			status = append(status, result)
			mx.Unlock()
		}(record)
	}
	wg.Wait()

	return status
}

// Incidents reads the CSV file containing the list of incidents and returns
// the list of incidents.
func (p StatusPage) Incidents() []string {
	logger := zerolog.Ctx(p.ctx).
		With().Str("component", "StatusPage.Incidents").
		Logger()

	var incidents []string
	if _, err := os.Stat(p.IncidentsPath); !errors.Is(err, os.ErrNotExist) {
		file, err := os.Open(p.IncidentsPath)
		if err != nil {
			logger.Error().Err(err).Send()
			return nil
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			incidents = append(incidents, strings.TrimSpace(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			logger.Error().Err(err).Msgf("failed to extract incidents from '%s'", p.IncidentsPath)
			return nil
		}
		_ = file.Close()
	}
	return incidents
}
