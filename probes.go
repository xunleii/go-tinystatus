package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Probe is used to scan a specific target and return its current status.
type Probe func(record *RecordStatus) Status

// Probes reprensents all probes defined into go-tinystatus
var (
	Probes = map[string]Probe{
		// tinystatus probe
		"http6": httpProbe,
		"http4": httpProbe,
		"http":  httpProbe,
	}
)

// httpProbe implements the scan probe for all HTTP records.
func httpProbe(record *RecordStatus) Status {
	if !strings.HasPrefix(record.Target, "http") {
		// NOTE: force to use a protocol scheme
		record.Target = "http://" + record.Target
	}

	expectedCode, err := strconv.Atoi(record.Expectation)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("Invalid expected status code '%s': should a number", record.Expectation),
		}
	}

	client := &http.Client{
		// NOTE: allow insecure HTTPS requests (not the goal of this scan)
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				network := "tcp4"
				if record.CType == "http6" {
					network = "tcp6"
				}

				return (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second, // NOTE: avoid infinite timeout
	}

	resp, err := client.Get(record.Target)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  errUnwrapAll(err),
		}
	}

	if resp.StatusCode != expectedCode {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("Status code: %d", resp.StatusCode),
		}
	}

	return Status{RecordStatus: record}
}

// errUnwrapAll returns the deepest error (origin) from wrapped errors.
// This is useful to avoid too much information to display on the status
// page.
func errUnwrapAll(werr error) error {
	err := errors.Unwrap(werr)
	if err != nil {
		return errUnwrapAll(err)
	}
	return werr
}
