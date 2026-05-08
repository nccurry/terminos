// SPDX-License-Identifier: AGPL-3.0-or-later

// Package replay renders captured demo streams back to a terminal.
package replay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
)

// Options controls terminal playback.
type Options struct {
	NoDelay   bool
	Speed     float64
	StepMode  string
	TypeDelay *time.Duration
	MaxDelay  *time.Duration
	Input     io.Reader
	Prompt    io.Writer
	Sleep     func(context.Context, time.Duration) error
}

// Play replays all captured steps in manifest order.
func Play(ctx context.Context, resolved bundle.Bundle, demo model.Demo, out io.Writer, opts Options) error {
	config, err := newConfig(demo, opts)
	if err != nil {
		return err
	}

	for index, step := range demo.Steps {
		if index > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
			}
		}
		if err := playStep(ctx, resolved, demo, step, out, config); err != nil {
			return err
		}
	}
	return nil
}

type config struct {
	noDelay   bool
	speed     float64
	stepMode  string
	typeDelay time.Duration
	maxDelay  time.Duration
	input     *bufio.Reader
	prompt    io.Writer
	sleep     func(context.Context, time.Duration) error
}

func newConfig(demo model.Demo, opts Options) (config, error) {
	speed := opts.Speed
	if speed == 0 {
		speed = 1
	}
	if speed <= 0 {
		return config{}, apperror.New(apperror.CodeUsage, "--speed must be greater than zero")
	}

	stepMode := opts.StepMode
	if strings.TrimSpace(stepMode) == "" {
		stepMode = demo.Playback.StepMode
	}
	switch stepMode {
	case "auto", "enter", "none":
	default:
		return config{}, apperror.Newf(apperror.CodeUsage, "--step-mode must be one of: auto, enter, none")
	}

	typeDelay := demo.Playback.TypeDelay.ToDuration()
	if opts.TypeDelay != nil {
		typeDelay = *opts.TypeDelay
	}
	if typeDelay < 0 {
		return config{}, apperror.New(apperror.CodeUsage, "--type-delay cannot be negative")
	}

	maxDelay := demo.Playback.MaxDelay.ToDuration()
	if opts.MaxDelay != nil {
		maxDelay = *opts.MaxDelay
	}
	if maxDelay < 0 {
		return config{}, apperror.New(apperror.CodeUsage, "--max-delay cannot be negative")
	}

	sleep := sleepReal
	if opts.Sleep != nil {
		sleep = opts.Sleep
	}

	var input *bufio.Reader
	if opts.Input != nil {
		input = bufio.NewReader(opts.Input)
	}

	return config{
		noDelay:   opts.NoDelay,
		speed:     speed,
		stepMode:  stepMode,
		typeDelay: typeDelay,
		maxDelay:  maxDelay,
		input:     input,
		prompt:    opts.Prompt,
		sleep:     sleep,
	}, nil
}

func playStep(ctx context.Context, resolved bundle.Bundle, demo model.Demo, step model.Step, out io.Writer, config config) error {
	events, err := capture.ReadEvents(capture.CapturePath(resolved, step))
	if err != nil {
		return err
	}

	command := step.Command
	for _, event := range events {
		if event.Type == model.EventStart && event.Command != "" {
			command = event.Command
			break
		}
	}

	if step.Title != "" {
		if _, err := fmt.Fprintf(out, "# %s\n", step.Title); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
		}
	}
	if err := typeLine(ctx, out, demo.Defaults.Prompt+command, config); err != nil {
		return err
	}
	if err := waitForStep(ctx, step, config); err != nil {
		return err
	}

	var previousTimeMS int64
	for _, event := range events {
		switch event.Type {
		case model.EventStdout, model.EventStderr:
			if err := waitForEvent(ctx, event, &previousTimeMS, config); err != nil {
				return err
			}
			if _, err := fmt.Fprint(out, event.Data); err != nil {
				return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
			}
		}
	}
	return nil
}

func typeLine(ctx context.Context, out io.Writer, line string, config config) error {
	delay := scaledDelay(config.typeDelay, config)
	if config.noDelay || delay <= 0 {
		_, err := fmt.Fprintln(out, line)
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
		}
		return nil
	}
	for _, char := range line {
		if _, err := fmt.Fprintf(out, "%c", char); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
		}
		if err := config.sleep(ctx, delay); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write playback output", err)
	}
	return nil
}

func waitForStep(ctx context.Context, step model.Step, config config) error {
	if config.noDelay || config.stepMode != "enter" || config.input == nil {
		return nil
	}
	if config.prompt != nil {
		label := step.ID
		if step.Title != "" {
			label = step.Title
		}
		if _, err := fmt.Fprintf(config.prompt, "Press Enter to show %s...\n", label); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write playback prompt", err)
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := config.input.ReadString('\n')
		if err == io.EOF {
			err = nil
		}
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return apperror.Wrap(apperror.CodeFilesystem, "read playback input", err)
		}
		return nil
	}
}

func waitForEvent(ctx context.Context, event model.CaptureEvent, previousTimeMS *int64, config config) error {
	if config.noDelay || config.stepMode != "auto" {
		if event.TimeMS > *previousTimeMS {
			*previousTimeMS = event.TimeMS
		}
		return nil
	}

	deltaMS := event.TimeMS - *previousTimeMS
	if deltaMS < 0 {
		deltaMS = 0
	}
	if event.TimeMS > *previousTimeMS {
		*previousTimeMS = event.TimeMS
	}

	delay := scaledDelay(time.Duration(deltaMS)*time.Millisecond, config)
	if delay > config.maxDelay {
		delay = config.maxDelay
	}
	return config.sleep(ctx, delay)
}

func scaledDelay(delay time.Duration, config config) time.Duration {
	if delay <= 0 || config.speed == 1 {
		return delay
	}
	return time.Duration(float64(delay) / config.speed)
}

func sleepReal(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
