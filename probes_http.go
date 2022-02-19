package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// ProbeHTTP is a probe implementation for HTTP services.
type ProbeHTTP struct{ *http.Transport }

// Sanitize implements the (Probe).Sanitize method.
func (p ProbeHTTP) Sanitize(check StatusCheck) (StatusCheck, error) {
	if !strings.HasPrefix(check.Target, "http") {
		// NOTE: force to use a protocol scheme
		check.Target = "http://" + check.Target
	}

	if _, err := url.Parse(check.Target); err != nil {
		return check, err
	}

	if _, err := strconv.Atoi(check.Expectation); err != nil {
		return check, fmt.Errorf("invalid expected status code '%s': should a number", check.Expectation)
	}

	return check, nil
}

// Scan implements the (Probe).Scan method.
func (p ProbeHTTP) Scan(ctx context.Context, check StatusCheck) ProbeResult {
	logger := zerolog.Ctx(ctx).With().
		Str("probe", check.CType).
		Str("target", check.Target).
		Logger()

	client := &http.Client{Timeout: ProbeTimeout, Transport: p.Transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.Target, nil)
	if err != nil {
		logger.Error().Err(err).Send()
		return errUnwrapAll(err)
	}

	logger.Trace().Msg("request sent")
	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Send()
		return errUnwrapAll(err)
	}
	_ = resp.Body.Close()

	expectedCode, _ := strconv.Atoi(check.Expectation)
	if resp.StatusCode != expectedCode {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		logger.Error().Err(err).Send()
		return err
	}

	return nil
}

func init() {
	DefaultTransport, _ := http.DefaultTransport.(*http.Transport)

	Probes["http4"] = ProbeHTTP{&http.Transport{
		Proxy:                 DefaultTransport.Proxy,
		DialContext:           DefaultTransport.DialContext,
		ForceAttemptHTTP2:     DefaultTransport.ForceAttemptHTTP2,
		MaxIdleConns:          DefaultTransport.MaxIdleConns,
		IdleConnTimeout:       DefaultTransport.IdleConnTimeout,
		TLSHandshakeTimeout:   DefaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: DefaultTransport.ExpectContinueTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}}
	Probes["http6"] = ProbeHTTP{&http.Transport{
		Proxy:                 DefaultTransport.Proxy,
		ForceAttemptHTTP2:     DefaultTransport.ForceAttemptHTTP2,
		MaxIdleConns:          DefaultTransport.MaxIdleConns,
		IdleConnTimeout:       DefaultTransport.IdleConnTimeout,
		TLSHandshakeTimeout:   DefaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: DefaultTransport.ExpectContinueTimeout,
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return DefaultTransport.DialContext(ctx, "tcp6", addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}}

	Probes["http"] = Probes["http4"]
}
