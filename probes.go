package main

import (
	"context"
	"errors"
	"time"
)

//nolint:gochecknoglobals
var (
	// Probes represents all probes defined into go-tinystatus
	Probes = map[string]Probe{}

	// ProbeTimeout is the default timeout for a probe
	ProbeTimeout = 10 * time.Second
)

type (
	// Probe is used to scan a specific target and return its current status.
	Probe interface {
		// Sanitize modify the check according to the probe in order to add or
		// remove some metadata.
		Sanitize(check StatusCheck) (StatusCheck, error)

		// Scan returns the current status of the target.
		Scan(ctx context.Context, check StatusCheck) ProbeResult
	}

	// ProbeResult is the result of a probe scan on a record
	ProbeResult error
)

// errUnwrapAll returns the deepest error (origin) from wrapped errors.
// This is useful to avoid too much information to display on the status
// page.
func errUnwrapAll(werr error) error {
	if err := errors.Unwrap(werr); err != nil {
		return errUnwrapAll(err)
	}
	return werr
}
