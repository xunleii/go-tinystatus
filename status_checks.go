package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type (
	// StatusCheck represents a check that go-tinystatus should run
	StatusCheck struct {
		Category, Name             string
		CType, Target, Expectation string
	}
	// StatusCheckList is a list of StatusCheck to easily analyze them
	// in parallel
	StatusCheckList []StatusCheck
)

// NewStatusCheck parse the give line into a valid StatusCheck
func NewStatusCheck(line string) (StatusCheck, error) {
	record := strings.Split(line, ",")
	if len(record) < 4 {
		return StatusCheck{}, fmt.Errorf("wrong number of fields")
	}

	ctype := strings.ToLower(strings.TrimSpace(record[0]))
	probe, exists := Probes[ctype]
	if !exists {
		return StatusCheck{}, fmt.Errorf("probe '%s' not supported by go-tinystatus", ctype)
	}

	check := StatusCheck{
		CType:    ctype,
		Category: "Uncategorized", Name: strings.TrimSpace(record[2]),
		Target: strings.TrimSpace(record[3]), Expectation: strings.TrimSpace(record[1]),
	}
	if len(record) >= 5 && strings.TrimSpace(record[4]) != "" {
		check.Category = strings.TrimSpace(record[4])
	}

	return probe.Sanitize(check)
}

// Run starts the check using the right probe.
func (c StatusCheck) Run(ctx context.Context) ProbeResult { return Probes[c.CType].Scan(ctx, c) }

// RunAll runs all checks in parallel safely.
func (l StatusCheckList) RunAll(ctx context.Context) StatusList {
	// NOTE: limit the number of check to 32 requests in parallel
	ratelimit, wg, mx := make(chan struct{}, 32), sync.WaitGroup{}, sync.Mutex{}

	statuses := StatusList{}
	for _, check := range l {
		wg.Add(1)
		ratelimit <- struct{}{}

		go func(check StatusCheck) {
			defer wg.Done()
			defer func() { <-ratelimit }()

			result := check.Run(ctx)

			mx.Lock()
			statuses = append(statuses, Status{check, result})
			mx.Unlock()
		}(check)
	}
	wg.Wait()

	return statuses
}
