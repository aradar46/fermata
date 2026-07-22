// Package cmd is fermata's cobra CLI. It mirrors act's flag conventions where it
// can (-W for the workflow file) so act users feel at home (plan §3).
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aradar46/fermata/internal/controller"
	"github.com/aradar46/fermata/internal/engine"
	"github.com/spf13/cobra"
)

// version is stamped at build time:
//
//	go build -ldflags "-X github.com/aradar46/fermata/cmd.version=v0.1.0"
//
// It stays "dev" for ordinary local builds so a bug report can distinguish a
// release from someone's working tree.
var version = "dev"

func newRootCmd() *cobra.Command {
	var (
		workflow      string
		event         string
		breaks        []string
		noBreakOnFail bool
		secrets       []string
		secretFile    string
		reuse         bool
		bind          bool
		platforms     []string
		matrixSpecs   []string
		jsonEvents    bool
		hold          time.Duration
	)

	root := &cobra.Command{
		Use:           "fermata",
		Short:         "A local debugger for GitHub Actions workflows",
		Long:          "fermata runs a GitHub Actions workflow locally on act's engine and\nadds breakpoints, an in-container shell, and retry-from-the-broken-step.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a workflow locally, pausing at breakpoints or on failure",
		RunE: func(_ *cobra.Command, _ []string) error {
			if workflow == "" {
				return fmt.Errorf("no workflow given: use -W <path-to-workflow.yml>")
			}

			// Signals are handled inside the REPL while paused (Ctrl-C ->
			// confirm, not kill). We intentionally do not install
			// signal-based context cancellation here.
			ctx := context.Background()

			secretMap, err := engine.LoadSecrets(secretFile, secrets)
			if err != nil {
				return err
			}

			platformMap, err := engine.ResolvePlatforms(platforms)
			if err != nil {
				return err
			}

			matrix, err := engine.ParseMatrix(matrixSpecs)
			if err != nil {
				return err
			}

			ctrl := controller.New(breaks, !noBreakOnFail)
			ctrl.SetWorkflowFile(workflow)
			ctrl.SetHold(hold)
			ctrl.SetWorkdirMode(engine.WorkdirFor(workflow), bind)

			// Machine-readable event stream on stdout, so fermata can be
			// wrapped by CI jobs, bots, or editor plugins without parsing
			// human-facing output. Job logs then go to stderr to keep the
			// JSONL stream clean.
			logSink := io.Writer(os.Stdout)
			if jsonEvents {
				ctrl.SetEventSink(os.Stdout)
				logSink = os.Stderr
			}

			// Route act's log output through a gate the REPL can quiesce.
			gate := ctrl.LogWriter(logSink)

			return engine.Run(ctx, engine.Options{
				WorkflowFile: workflow,
				EventName:    event,
				Platforms:    platformMap,
				OnStep:       ctrl.OnStep,
				LogTo:        gate,
				OnCancel:     ctrl.SetCancel,
				Secrets:      secretMap,
				Reuse:        reuse,
				Bind:         bind,
				Matrix:       matrix,
				Notify: func(msg string) {
					fmt.Fprintln(os.Stderr, msg)
				},
			})
		},
	}

	runCmd.Flags().StringVarP(&workflow, "workflow", "W", "", "path to the workflow file to run (required)")
	runCmd.Flags().StringVar(&event, "event", "push", "GitHub event name to run the workflow for")
	runCmd.Flags().StringArrayVar(&breaks, "break", nil, "pause at a step by id or name (repeatable)")
	runCmd.Flags().BoolVar(&noBreakOnFail, "no-break-on-failure", false, "do not auto-pause when a step fails")
	runCmd.Flags().StringArrayVarP(&secrets, "secret", "s", nil, "secret as KEY=value, or bare KEY to take it from the environment (repeatable)")
	runCmd.Flags().StringVar(&secretFile, "secret-file", "", "file of KEY=value secrets (dotenv style)")
	runCmd.Flags().DurationVar(&hold, "hold", 0, "instead of an interactive prompt, keep a failed step's container alive for this long and print how to `docker exec` into it (e.g. --hold 30m); for runs with no terminal")
	runCmd.Flags().BoolVar(&jsonEvents, "json", false, "emit machine-readable JSONL events on stdout (job logs move to stderr)")
	runCmd.Flags().StringArrayVarP(&platforms, "platform", "P", nil, "map a runs-on label to a container image, e.g. -P ubuntu-latest=myimage:tag (repeatable; also how you map self-hosted labels)")
	runCmd.Flags().StringArrayVar(&matrixSpecs, "matrix", nil, "run only matching matrix legs, e.g. --matrix python:3.12 (repeatable)")
	runCmd.Flags().BoolVar(&bind, "bind", false, "bind-mount the working directory instead of copying it, so edits you make while paused are visible to `retry`")
	runCmd.Flags().BoolVar(&reuse, "reuse", false, "keep the job container after the run so tool caches (Gradle, Maven, pip) survive to the next run")

	root.AddCommand(runCmd)
	return root
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "fermata: %v\n", err)
		return 1
	}
	return 0
}
