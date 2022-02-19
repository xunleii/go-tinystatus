package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-ping/ping"
	"github.com/rs/zerolog"
)

// PingProbe is a probe implementation for ping targets.
type PingProbe struct{}

// Sanitize implements the (Probe).Sanitize method.
func (p PingProbe) Sanitize(check StatusCheck) (StatusCheck, error) {
	if _, err := strconv.Atoi(check.Expectation); err != nil {
		return check, fmt.Errorf("invalid expected return code '%s': should a number", check.Expectation)
	}

	return check, nil
}

// Scan implements the (Probe).Scan method.
func (p PingProbe) Scan(ctx context.Context, check StatusCheck) ProbeResult {
	logger := zerolog.Ctx(ctx).With().
		Str("probe", check.CType).
		Str("target", check.Target).
		Logger()

	expectedReturn, _ := strconv.Atoi(check.Expectation)
	shouldBePingable := expectedReturn == 0

	pinger, err := ping.NewPinger(check.Target)
	if err != nil {
		logger.Error().Err(err).Send()
		return err
	}
	pinger.Timeout = ProbeTimeout
	pinger.Count = 1

	logger.Trace().Msg("ping sent")
	err = pinger.Run()
	if shouldBePingable && err != nil {
		return errUnwrapAll(err)
	}

	pcktReceived := pinger.Statistics().PacketsRecv

	switch {
	case shouldBePingable && pcktReceived == 0:
		err = fmt.Errorf("no packet received")
		logger.Error().Err(err).Send()
		return err
	case !shouldBePingable && pcktReceived > 0:
		err = fmt.Errorf("`%s` should failed but succeed", check.CType)
		logger.Error().Err(err).Send()
		return err
	}

	return nil
}

func init() {
	Probes["ping"] = PingProbe{}
	Probes["ping4"] = PingProbe{}
}
