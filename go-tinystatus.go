package main

import (
	"bufio"
	"bytes"
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

var log = zerolog.New(os.Stderr).
	With().Timestamp().
	Logger()

func main() {
	var (
		checkPath     = "checks.csv"
		incidentsPath = "incidents.txt"

		daemonize = false
		addr      = ":8080"
		interval  = 15 * time.Second
	)
	flaggy.DefaultParser.DisableShowVersionWithVersion()

	flaggy.AddPositionalValue(&checkPath, checkPath, 1, false, "File containing all checks, formatted in CSV")
	flaggy.AddPositionalValue(&incidentsPath, incidentsPath, 2, false, "File containing all incidents to be displayed")

	flaggy.Duration(&timeout, "", "timeout", "Maximum time to wait a probe before aborting.")
	flaggy.Bool(&daemonize, "", "daemon", "Start go-tinystatus as daemon with an embedded web server.")
	flaggy.String(&addr, "", "addr", "Address on which the daemon will be listening.")
	flaggy.Duration(&interval, "", "interval", "Interval between two page rendering.")

	flaggy.Parse()

	var page StatusPage

	{
		file, err := os.Open(checkPath)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		scanner, line, nline := bufio.NewScanner(file), "", 0
		for scanner.Scan() {
			line, nline = strings.TrimSpace(scanner.Text()), nline+1
			if line == "" || strings.HasPrefix(line, "#") {
				// NOTE: ignore empty or commented lines
				continue
			}

			record := strings.Split(line, ",")
			if len(record) < 4 {
				log.Error().Msgf("invalid CSV row %d: wrong number of fields", nline)
				continue
			}

			ctype := strings.ToLower(strings.TrimSpace(record[0]))
			if _, exists := Probes[ctype]; !exists {
				log.Warn().Msgf("unknown probe '%s'", ctype)
				continue
			}

			rs := RecordStatus{
				CType:    ctype,
				Category: "Services", Name: strings.TrimSpace(record[2]),
				Target: strings.TrimSpace(record[3]), Expectation: strings.TrimSpace(record[1]),
			}
			if len(record) >= 5 {
				rs.Category = strings.TrimSpace(record[4])
			}

			page.Records = append(page.Records, rs)
		}

		if err := scanner.Err(); err != nil {
			log.Fatal().Err(err).Msgf("failed to extract CSV rows from '%s'", checkPath)
		}
		_ = file.Close()
	}

	if _, err := os.Stat(incidentsPath); !errors.Is(err, os.ErrNotExist) {
		file, err := os.Open(incidentsPath)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			page.Incidents = append(page.Incidents, strings.TrimSpace(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			log.Fatal().Err(err).Msgf("failed to extract incidents from '%s'", checkPath)
		}
		_ = file.Close()
	}

	html, err := page.RenderHTML()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render the status page")
	}

	if !daemonize {
		fmt.Print(html)
		return
	}

	rwx, ticker := sync.RWMutex{}, time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				page, err := page.RenderHTML()
				if err != nil {
					log.Error().Err(err).Msg("failed to render the status page")
				}

				rwx.Lock()
				html = page
				rwx.Unlock()
			}
		}
	}()

	log.Info().Msgf("start go-tinystatus listening on '%s'", addr)
	err = http.ListenAndServe(addr, http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
		rwx.RLock()
		defer rwx.RUnlock()
		_, _ = wr.Write([]byte(html))
	}))
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

// StatusPage contains all data required to generate the status page.
type StatusPage struct {
	Records   []RecordStatus
	Incidents []string

	start  time.Time
	status StatusList
}

// RenderHTML runs all checks in parallel and generate the HTML page.
func (p StatusPage) RenderHTML() (string, error) {
	p.start = time.Now()
	p.status = StatusList{}

	// NOTE: limit the number of check to 32 requests in parallel
	ratelimit, wg, mx := make(chan struct{}, 32), sync.WaitGroup{}, sync.Mutex{}
	for _, record := range p.Records {
		wg.Add(1)
		ratelimit <- struct{}{}

		go func(record RecordStatus) {
			defer wg.Done()
			defer func() { <-ratelimit }()
			result := Probes[record.CType](&record)

			mx.Lock()
			p.status = append(p.status, result)
			mx.Unlock()
		}(record)
	}
	wg.Wait()

	buff := bytes.NewBufferString("")
	err := templatedHtml.ExecuteTemplate(buff, "tinystatus", p)
	return buff.String(), err
}

func (p StatusPage) Status() StatusList     { return p.status }
func (p StatusPage) LastCheck() time.Time   { return time.Now() }
func (p StatusPage) Elapsed() time.Duration { return time.Since(p.start) }
