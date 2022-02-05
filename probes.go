package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-ping/ping"
)

// Probe is used to scan a specific target and return its current status.
type Probe func(record *RecordStatus) Status

// Probes represents all probes defined into go-tinystatus
var Probes = map[string]Probe{
	// tinystatus probes
	"http6": httpProbe,
	"http4": httpProbe,
	"http":  httpProbe,
	"ping6": pingProbe,
	"ping4": pingProbe,
	"ping":  pingProbe,
	"port6": portProbe,
	"port4": portProbe,
	"port":  portProbe,

	// go-tinystatus probes
	"tcp6": portProbe,
	"tcp4": portProbe,
	"tcp":  portProbe,
}

var (
	timeout      = 10 * time.Second
	rxPortTarget = regexp.MustCompile(`(?P<host>[^\s]+)\s+(?P<port>\d+)`)
)

// httpProbe implements the scan probe for all `HTTP` records.
func httpProbe(record *RecordStatus) Status {
	if !strings.HasPrefix(record.Target, "http") {
		// NOTE: force to use a protocol scheme
		record.Target = "http://" + record.Target
	}

	expectedCode, err := strconv.Atoi(record.Expectation)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("invalid expected status code '%s': should a number", record.Expectation),
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
					Timeout:   timeout,
					KeepAlive: timeout,
				}).DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: timeout, // NOTE: avoid infinite timeout
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
			ProbeResult:  fmt.Errorf("status code: %d", resp.StatusCode),
		}
	}

	return Status{RecordStatus: record}
}

// pingProbe implements the scan for all `PING` records.
func pingProbe(record *RecordStatus) Status {
	if record.CType == "ping6" {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("`ping6` is not supported by go-tinystatus"),
		}
	}

	expectedReturn, err := strconv.Atoi(record.Expectation)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("invalid expected return code '%s': should a number", record.Expectation),
		}
	}
	shouldBePingable := expectedReturn == 0

	pinger, err := ping.NewPinger(record.Target)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  err,
		}
	}
	pinger.Timeout = timeout
	pinger.Count = 1

	err = pinger.Run()
	if shouldBePingable && err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  err,
		}
	}

	pcktReceived := pinger.Statistics().PacketsRecv

	status := Status{RecordStatus: record}
	switch {
	case shouldBePingable && pcktReceived == 0:
		status.ProbeResult = fmt.Errorf("no packet received")
	case !shouldBePingable && pcktReceived > 0:
		status.ProbeResult = fmt.Errorf("`%s` should failed but succeed", record.CType)
	}
	return status
}

// portProbe implements the scan for all `PORT` or `TCP` records.
func portProbe(record *RecordStatus) Status {
	expectedReturn, err := strconv.Atoi(record.Expectation)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("invalid expected return code '%s': should a number", record.Expectation),
		}
	}
	shouldBeOpen := expectedReturn == 0

	addr := rxPortTarget.ReplaceAllString(record.Target, "$host:$port")
	if addr == record.Target {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("invalid target '%s': should be formated like '<host> <port>'", record.Target),
		}
	}

	network := strings.ReplaceAll(record.CType, "port", "tcp") // NOTE: convert portX in tcpX
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil && (shouldBeOpen || !err.(net.Error).Timeout()) {
		return Status{
			RecordStatus: record,
			ProbeResult:  err,
		}
	}

	status := Status{RecordStatus: record}
	switch {
	case shouldBeOpen && conn == nil:
		host, port, _ := net.SplitHostPort(addr)
		status.ProbeResult = fmt.Errorf("connect to %s port %s (%s) failed: Connection refused", host, port, network)
	case !shouldBeOpen && conn != nil:
		host, port, _ := net.SplitHostPort(addr)
		status.ProbeResult = fmt.Errorf("connect to %s port %s (%s) succeed", host, port, network)
	}
	return status
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
