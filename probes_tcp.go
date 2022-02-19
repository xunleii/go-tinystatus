package main

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// TCPProbe is a probe implementation for TCP services.
type TCPProbe struct{ rxTarget *regexp.Regexp }

// Sanitize implements the (Probe).Sanitize method.
func (p TCPProbe) Sanitize(check StatusCheck) (StatusCheck, error) {
	if _, err := strconv.Atoi(check.Expectation); err != nil {
		return check, fmt.Errorf("invalid expected return code '%s': should a number", check.Expectation)
	}

	return check, nil
}

// Scan implements the (Probe).Scan method.
func (p TCPProbe) Scan(ctx context.Context, check StatusCheck) ProbeResult {
	logger := zerolog.Ctx(ctx).With().
		Str("probe", check.CType).
		Str("target", check.Target).
		Logger()

	expectedReturn, _ := strconv.Atoi(check.Expectation)
	shouldBeOpen := expectedReturn == 0

	addr := p.rxTarget.ReplaceAllString(check.Target, "$host:$port")
	if addr == check.Target {
		err := fmt.Errorf("invalid target '%s': should be formated like '<host> <port>'", check.Target)
		logger.Error().Err(err).Send()
		return err
	}

	logger.Trace().Msg("port scan sent")
	network := strings.ReplaceAll(check.CType, "port", "tcp") // NOTE: convert portX in tcpX
	conn, err := net.DialTimeout(network, addr, ProbeTimeout)
	if err != nil && (shouldBeOpen || !err.(net.Error).Timeout()) { //nolint // net.DialTimeout only returns net.Error
		logger.Error().Err(err).Send()
		return errUnwrapAll(err)
	}

	switch {
	case shouldBeOpen && conn == nil:
		host, port, _ := net.SplitHostPort(addr)
		err = fmt.Errorf("connect to %s port %s (%s) failed: Connection refused", host, port, network)
		logger.Error().Err(err).Send()
		return err
	case !shouldBeOpen && conn != nil:
		host, port, _ := net.SplitHostPort(addr)
		err = fmt.Errorf("connect to %s port %s (%s) succeed", host, port, network)
		logger.Error().Err(err).Send()
		return err
	}

	return nil
}

func init() {
	Probes["tcp"] = TCPProbe{rxTarget: regexp.MustCompile(`(?P<host>[^\s]+)\s+(?P<port>\d+)`)}
	Probes["tcp4"] = Probes["tcp"]
	Probes["tcp6"] = Probes["tcp"]

	// NOTE: keep compatibility with tinystatus
	Probes["port"] = Probes["tcp"]
	Probes["port4"] = Probes["tcp"]
	Probes["port6"] = Probes["tcp"]
}
