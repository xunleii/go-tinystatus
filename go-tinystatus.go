package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/integrii/flaggy"
	"github.com/rs/zerolog"
)

func main() {
	page := StatusPage{
		statusChecksPath: "checks.csv",
		incidentsPath:    "incidents.txt",
	}

	var (
		daemonize = false
		addr      = ":8080"
		interval  = 15 * time.Second
		logLevel  = zerolog.LevelInfoValue
	)

	flaggy.DefaultParser.DisableShowVersionWithVersion()
	flaggy.AddPositionalValue(&page.statusChecksPath, page.statusChecksPath, 1, false, "File containing all checks, formatted in CSV")
	flaggy.AddPositionalValue(&page.incidentsPath, page.incidentsPath, 2, false, "File containing all incidents to be displayed")
	flaggy.Duration(&ProbeTimeout, "", "timeout", "Maximum time to wait a probe before aborting.")
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
	if lvl <= zerolog.DebugLevel {
		logger = logger.With().Caller().Logger()
	}

	ctx, done := context.WithCancel(logger.WithContext(context.Background()))

	html, err := page.RenderHTML(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to render the status page")
	}

	if !daemonize {
		done()
		fmt.Print(html)
		return
	}

	rwx, ticker := sync.RWMutex{}, time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				newHTML, rendErr := page.RenderHTML(ctx)
				if rendErr != nil {
					logger.Error().Err(rendErr).Msg("failed to render the status page")
				}

				rwx.Lock()
				html = newHTML
				rwx.Unlock()

			case <-ctx.Done():
				return
			}
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
