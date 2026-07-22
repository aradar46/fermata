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
		Long: "Run a workflow locally, pausing at breakpoints or on failure.\n\n" +
			"At a pause the job container is still alive. Type `shell` to look\n" +
			"inside it, fix the problem, then `retry` to re-run only that step.",
		Example: "  # Pause wherever a step fails\n" +
			"  fermata run -W .github/workflows/ci.yml\n\n" +
			"  # Fix source files while paused and have retry pick them up\n" +
			"  fermata run -W .github/workflows/ci.yml --bind\n\n" +
			"  # Stop at a specific step instead of waiting for a failure\n" +
			"  fermata run -W .github/workflows/ci.yml --break \"Run tests\"\n\n" +
			"  # A workflow that does not trigger on push, with secrets\n" +
			"  fermata run -W .github/workflows/deploy.yml \\\n" +
			"      --event workflow_dispatch --secret-file .secrets\n\n" +
			"  # One matrix leg, on an image that has your tooling\n" +
			"  fermata run -W .github/workflows/ci.yml \\\n" +
			"      --matrix python:3.12 -P ubuntu-latest=my/image:tag",
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

	registerCompletions(runCmd, &workflow)

	root.AddCommand(runCmd)
	return root
}

// registerCompletions teaches the shell to suggest values, not just flag names.
//
// Without this, `--break <TAB>` offers filenames, which is actively misleading:
// the flag takes a step id or name, and the only way to get one right is to
// open the YAML and copy a string with spaces in it exactly. workflow points at
// the -W value so step completion can read the file the user already chose.
func registerCompletions(runCmd *cobra.Command, workflow *string) {
	noFiles := cobra.ShellCompDirectiveNoFileComp

	_ = runCmd.RegisterFlagCompletionFunc("workflow",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			files := engine.WorkflowFiles(".")
			if len(files) == 0 {
				// No .github/workflows here: fall back to normal file
				// completion rather than insisting the directory is empty.
				return nil, cobra.ShellCompDirectiveDefault
			}
			return files, noFiles
		})

	_ = runCmd.RegisterFlagCompletionFunc("break",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			wf := *workflow
			if wf == "" {
				// -W has not been typed yet. One workflow in the repo means
				// there is no ambiguity about which file to read.
				if files := engine.WorkflowFiles("."); len(files) == 1 {
					wf = files[0]
				}
			}
			if wf == "" {
				return nil, noFiles
			}
			return engine.StepNames(wf), noFiles
		})

	_ = runCmd.RegisterFlagCompletionFunc("event",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			if wf := *workflow; wf != "" {
				if events := engine.WorkflowEvents(wf); len(events) > 0 {
					return events, noFiles
				}
			}
			return []string{
				"push\tthe default",
				"pull_request",
				"workflow_dispatch\tmanually triggered workflows",
				"schedule",
				"release",
			}, noFiles
		})

	_ = runCmd.RegisterFlagCompletionFunc("secret-file",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{"secrets", "env"}, cobra.ShellCompDirectiveFilterFileExt
		})

	_ = runCmd.RegisterFlagCompletionFunc("hold",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{"10m", "30m", "1h"}, noFiles
		})

	// Flags whose values are freeform (KEY=value, label=image, key:value) get
	// no candidates, but must still suppress filename completion: offering
	// files for `--secret` would only ever be wrong.
	for _, name := range []string{"secret", "platform", "matrix"} {
		_ = runCmd.RegisterFlagCompletionFunc(name,
			func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
				return nil, noFiles
			})
	}
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "fermata: %v\n", err)
		return 1
	}
	return 0
}
