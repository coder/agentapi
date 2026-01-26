package util

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/coder/quartz"
	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/xerrors"
)

type WaitTimeout struct {
	Timeout     time.Duration
	MinInterval time.Duration
	MaxInterval time.Duration
	InitialWait bool
	Clock       quartz.Clock
}

var WaitTimedOut = xerrors.New("timeout waiting for condition")

// WaitFor waits for a condition to be true or the timeout to expire.
// It will wait for the condition to be true with exponential backoff.
func WaitFor(ctx context.Context, timeout WaitTimeout, condition func() (bool, error)) error {
	clock := timeout.Clock
	if clock == nil {
		clock = quartz.NewReal()
	}

	minInterval := timeout.MinInterval
	maxInterval := timeout.MaxInterval
	timeoutDuration := timeout.Timeout
	if minInterval == 0 {
		minInterval = 10 * time.Millisecond
	}
	if maxInterval == 0 {
		maxInterval = 500 * time.Millisecond
	}
	if timeoutDuration == 0 {
		timeoutDuration = 10 * time.Second
	}
	timeoutTimer := clock.NewTimer(timeoutDuration)
	defer timeoutTimer.Stop()

	if minInterval > maxInterval {
		return xerrors.Errorf("minInterval is greater than maxInterval")
	}

	interval := minInterval
	if timeout.InitialWait {
		initialTimer := clock.NewTimer(interval)
		select {
		case <-initialTimer.C:
		case <-ctx.Done():
			initialTimer.Stop()
			return ctx.Err()
		case <-timeoutTimer.C:
			initialTimer.Stop()
			return WaitTimedOut
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTimer.C:
			return WaitTimedOut
		default:
			ok, err := condition()
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
			sleepTimer := clock.NewTimer(interval)
			select {
			case <-sleepTimer.C:
			case <-ctx.Done():
				sleepTimer.Stop()
				return ctx.Err()
			case <-timeoutTimer.C:
				sleepTimer.Stop()
				return WaitTimedOut
			}
			interval = min(interval*2, maxInterval)
		}
	}
}

// based on https://github.com/danielgtaylor/huma/issues/621#issuecomment-2456588788
func OpenAPISchema[T ~string](r huma.Registry, enumName string, values []T) *huma.Schema {
	if r.Map()[enumName] == nil {
		schemaRef := r.Schema(reflect.TypeOf(""), true, enumName)
		schemaRef.Title = enumName
		schemaRef.Examples = []any{values[0]}
		for _, v := range values {
			schemaRef.Enum = append(schemaRef.Enum, string(v))
		}
		r.Map()[enumName] = schemaRef
	}
	return &huma.Schema{Ref: fmt.Sprintf("#/components/schemas/%s", enumName)}
}
