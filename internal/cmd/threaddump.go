package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const defaultMaxStackFrames = 10

var validThreadStates = []string{"NEW", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "TERMINATED"}

type threaddumpCommandOperations struct {
	baseOperations
	output       string
	stateFilter  string
	nameFilter   string
	summary      bool
	noStacktrace bool
}

func NewThreadDumpCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &threaddumpCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:     "threaddump",
		Aliases: []string{"thread-dump"},
		Short:   "Get thread dump and analyze thread states",
		Long: `Get thread dump from Spring Boot Actuator.

Displays thread information including thread states, blocked threads, and stack traces.`,
		Example: `  # Show only the thread state summary
  kubectl actuator -d my-app threaddump --summary

  # Show blocked threads with their stack traces
  kubectl actuator -d my-app threaddump --state BLOCKED

  # Show threads whose name contains 'http', without stack traces
  kubectl actuator -d my-app threaddump -f http --no-stacktrace`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get threaddump", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	addOutputFlag(cmd, &operations.output, "", OutputFormatWide, OutputFormatJSON, OutputFormatYAML)
	cmd.Flags().StringVar(&operations.stateFilter, "state", "", "Filter by thread state (e.g., BLOCKED, WAITING, RUNNABLE)")
	_ = cmd.RegisterFlagCompletionFunc("state", cobra.FixedCompletions(validThreadStates, cobra.ShellCompDirectiveNoFileComp))
	cmd.Flags().StringVarP(&operations.nameFilter, "filter", "f", "", "Filter by thread name (case-insensitive substring)")
	markNoFileFlags(cmd, "filter")
	cmd.Flags().BoolVar(&operations.summary, "summary", false, "Show only thread state summary")
	cmd.Flags().BoolVar(&operations.noStacktrace, "no-stacktrace", false, "Show thread list without stack traces")

	return cmd
}

func (o *threaddumpCommandOperations) validateFlags() error {
	if err := validateOutputFormat(o.output, OutputFormatWide, OutputFormatJSON, OutputFormatYAML); err != nil {
		return err
	}

	if isStructuredOutput(o.output) && (o.summary || o.noStacktrace) {
		return fmt.Errorf("--summary and --no-stacktrace cannot be combined with -o %s", o.output)
	}

	// State matching is case-insensitive throughout, so the filter is only
	// validated here, never rewritten.
	if o.stateFilter != "" && !slices.Contains(validThreadStates, strings.ToUpper(o.stateFilter)) {
		return fmt.Errorf("invalid thread state %q\nValid states: %s", o.stateFilter, strings.Join(validThreadStates, ", "))
	}

	return nil
}

func (o *threaddumpCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	data, err := client.GetRaw("threaddump")
	if err != nil {
		return nil, err
	}
	return filterThreadDumpJSON(data, o.stateFilter, o.nameFilter)
}

// filterThreadDumpJSON applies the state and name filters to the raw
// /threaddump response, keeping all other thread fields intact.
func filterThreadDumpJSON(data json.RawMessage, stateFilter, nameFilter string) (json.RawMessage, error) {
	if stateFilter == "" && nameFilter == "" {
		return data, nil
	}
	tree, err := decodeTree(data)
	if err != nil {
		return nil, err
	}
	threads, ok := tree["threads"].([]any)
	if !ok {
		return data, nil
	}
	filtered := make([]any, 0, len(threads))
	for _, threadValue := range threads {
		threadMap, ok := threadValue.(map[string]any)
		if !ok {
			continue
		}
		state, _ := threadMap["threadState"].(string)
		name, _ := threadMap["threadName"].(string)
		if stateFilter != "" && !strings.EqualFold(state, stateFilter) {
			continue
		}
		if !matchesFilter(name, nameFilter) {
			continue
		}
		filtered = append(filtered, threadValue)
	}
	tree["threads"] = filtered
	return encodeTree(tree)
}

func (o *threaddumpCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	threaddump, err := client.GetThreadDump()
	if err != nil {
		return err
	}

	return o.displayThreadDump(threaddump)
}

func (o *threaddumpCommandOperations) displayThreadDump(threaddump *actuator.ThreadDumpResponse) error {
	filteredThreads, stateCounts := o.filterThreads(threaddump.Threads)

	displayThreadSummary(len(threaddump.Threads), stateCounts)

	if o.summary {
		return nil
	}

	fmt.Println()

	if len(filteredThreads) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "No threads match the specified filters")
		return nil
	}

	if len(filteredThreads) < len(threaddump.Threads) {
		fmt.Printf("Showing %d filtered threads:\n\n", len(filteredThreads))
	}

	opts := threadDisplayOptions{wide: o.output == OutputFormatWide, noStacktrace: o.noStacktrace}
	for i, thread := range filteredThreads {
		displayThread(thread, i+1, opts)
	}

	return nil
}

// threadDisplayOptions controls how much detail displayThread renders.
type threadDisplayOptions struct {
	wide         bool
	noStacktrace bool
}

// maxFrames returns the stack frame limit; wide mode shows all frames.
func (opts threadDisplayOptions) maxFrames() int {
	if opts.wide {
		return -1
	}
	return defaultMaxStackFrames
}

func (o *threaddumpCommandOperations) filterThreads(threads []actuator.Thread) ([]actuator.Thread, map[string]int) {
	var filtered []actuator.Thread
	stateCounts := make(map[string]int)

	for _, thread := range threads {
		stateCounts[thread.ThreadState]++

		if o.stateFilter != "" && !strings.EqualFold(thread.ThreadState, o.stateFilter) {
			continue
		}
		if !matchesFilter(thread.ThreadName, o.nameFilter) {
			continue
		}
		filtered = append(filtered, thread)
	}

	return filtered, stateCounts
}

func displayThreadSummary(totalThreads int, stateCounts map[string]int) {
	fmt.Printf("Total Threads: %d\n", totalThreads)
	fmt.Println("\nThread States:")
	for _, state := range validThreadStates {
		if count, exists := stateCounts[state]; exists {
			fmt.Printf("  %s: %d\n", state, count)
		}
	}
}

func displayThread(thread actuator.Thread, index int, opts threadDisplayOptions) {
	fmt.Printf("Thread #%d: %s (ID: %d)\n", index, thread.ThreadName, thread.ThreadID)
	fmt.Printf("  State: %s\n", thread.ThreadState)
	fmt.Printf("  Daemon: %t, In Native: %t, Suspended: %t\n", thread.Daemon, thread.InNative, thread.Suspended)

	if thread.Priority > 0 && opts.wide {
		fmt.Printf("  Priority: %d\n", thread.Priority)
	}

	if thread.BlockedCount > 0 {
		fmt.Printf("  Blocked Count: %d", thread.BlockedCount)
		if thread.BlockedTime > 0 {
			fmt.Printf(", Time: %d ms", thread.BlockedTime)
		}
		fmt.Println()
	}

	if thread.WaitedCount > 0 {
		fmt.Printf("  Waited Count: %d", thread.WaitedCount)
		if thread.WaitedTime > 0 {
			fmt.Printf(", Time: %d ms", thread.WaitedTime)
		}
		fmt.Println()
	}

	if thread.LockOwnerID > 0 {
		fmt.Printf("  Waiting on lock owned by thread ID: %d\n", thread.LockOwnerID)
	}

	if !opts.noStacktrace && len(thread.StackTrace) > 0 {
		displayStackTrace(thread.StackTrace, opts.maxFrames())
	}

	fmt.Println()
}

func displayStackTrace(frames []actuator.StackFrame, maxFrames int) {
	fmt.Println("  Stack Trace:")

	framesToShow := len(frames)
	if maxFrames > 0 && framesToShow > maxFrames {
		framesToShow = maxFrames
	}

	for i := 0; i < framesToShow; i++ {
		frame := frames[i]
		fmt.Printf("    at %s.%s(%s)\n", frame.ClassName, frame.MethodName, formatFrameLocation(frame))
	}

	if len(frames) > framesToShow {
		fmt.Printf("    ... %d more frames\n", len(frames)-framesToShow)
	}
}

func formatFrameLocation(frame actuator.StackFrame) string {
	if frame.FileName != nil {
		if frame.LineNumber != nil && *frame.LineNumber != -1 {
			return fmt.Sprintf("%s:%d", *frame.FileName, *frame.LineNumber)
		}
		return *frame.FileName
	}
	if frame.NativeMethod {
		return "Native Method"
	}
	return "Unknown Source"
}
