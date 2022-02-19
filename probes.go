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
type Probe func(ctx context.Context, record *RecordStatus) Status

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

	ipv4Transport = &http.Transport{
		Proxy:                 http.DefaultTransport.(*http.Transport).Proxy,
		DialContext:           http.DefaultTransport.(*http.Transport).DialContext,
		ForceAttemptHTTP2:     http.DefaultTransport.(*http.Transport).ForceAttemptHTTP2,
		MaxIdleConns:          http.DefaultTransport.(*http.Transport).MaxIdleConns,
		IdleConnTimeout:       http.DefaultTransport.(*http.Transport).IdleConnTimeout,
		TLSHandshakeTimeout:   http.DefaultTransport.(*http.Transport).TLSHandshakeTimeout,
		ExpectContinueTimeout: http.DefaultTransport.(*http.Transport).ExpectContinueTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // NOTE: disable TLS verification
	}
	ipv6Transport = &http.Transport{
		Proxy:                 http.DefaultTransport.(*http.Transport).Proxy,
		ForceAttemptHTTP2:     http.DefaultTransport.(*http.Transport).ForceAttemptHTTP2,
		MaxIdleConns:          http.DefaultTransport.(*http.Transport).MaxIdleConns,
		IdleConnTimeout:       http.DefaultTransport.(*http.Transport).IdleConnTimeout,
		TLSHandshakeTimeout:   http.DefaultTransport.(*http.Transport).TLSHandshakeTimeout,
		ExpectContinueTimeout: http.DefaultTransport.(*http.Transport).ExpectContinueTimeout,
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return http.DefaultTransport.(*http.Transport).DialContext(ctx, "tcp6", addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // NOTE: disable TLS verification
	}
)

// httpProbe implements the scan probe for all `HTTP` records.
func httpProbe(ctx context.Context, record *RecordStatus) Status {
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

	client := &http.Client{Timeout: timeout}
	client.Transport = ipv4Transport
	if record.CType == "http6" {
		client.Transport = ipv6Transport
	}

	req, err := http.NewRequest(http.MethodGet, record.Target, nil)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  errUnwrapAll(err),
		}
	}

	req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return Status{
			RecordStatus: record,
			ProbeResult:  errUnwrapAll(err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedCode {
		return Status{
			RecordStatus: record,
			ProbeResult:  fmt.Errorf("status code: %d", resp.StatusCode),
		}
	}

	return Status{RecordStatus: record}
}

// pingProbe implements the scan for all `PING` records.
func pingProbe(_ context.Context, record *RecordStatus) Status {
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
			ProbeResult:  errUnwrapAll(err),
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
func portProbe(ctx context.Context, record *RecordStatus) Status {
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
			ProbeResult:  errUnwrapAll(err),
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
