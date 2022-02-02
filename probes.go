package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Probe is used to scan a specific target and return its current status.
type Probe func(record RecordStatus) Status

// Probes reprensents all probes defined into go-tinystatus
var (
	Probes = map[string]Probe{
		// tinystatus probe
		"http6": httpProbe,
		"http4": httpProbe,
		"http":  httpProbe,
	}
)

func httpProbe(record RecordStatus) Status {
	expectedCode, err := strconv.Atoi(record.Expectation)
	if err != nil {
		return Status{
			RecordStatus: &record,
			ProbeResult:  fmt.Errorf("Invalid status code '%s': should a number", record.Expectation),
		}
	}

	client := &http.Client{
		// NOTE: allow insecure HTTPS requests (not the goal of this scan)
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   10 * time.Second, // NOTE: avoid infinite timeout
	}

	resp, err := client.Get(record.Target)
	if err != nil {
		return Status{
			RecordStatus: &record,
			ProbeResult:  err,
		}
	}

	if resp.StatusCode != expectedCode {
		return Status{
			RecordStatus: &record,
			ProbeResult:  fmt.Errorf("Status code: %d", resp.StatusCode),
		}
	}

	return Status{RecordStatus: &record}
}
