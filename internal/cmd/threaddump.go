package cmd

import (
	"context"
	"fmt"
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
	wideMode     bool
}

func NewThreadDumpCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &threaddumpCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "threaddump",
		Short: "Get thread dump and analyze thread states",
		Long: `Get thread dump from Spring Boot Actuator.

Displays thread information including thread states, blocked threads, and stack traces.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get threaddump", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: wide")
	cmd.Flags().StringVar(&operations.stateFilter, "state", "", "Filter by thread state (e.g., BLOCKED, WAITING, RUNNABLE)")
	cmd.Flags().StringVar(&operations.nameFilter, "name", "", "Filter by thread name pattern")
	cmd.Flags().BoolVar(&operations.summary, "summary", false, "Show only thread state summary")
	cmd.Flags().BoolVar(&operations.noStacktrace, "no-stacktrace", false, "Show thread list without stack traces")

	return cmd
}

func (o *threaddumpCommandOperations) complete(cmd *cobra.Command) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}
	o.wideMode = o.output == OutputFormatWide
	return nil
}

func (o *threaddumpCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}

	if err := validateOutputFormat(o.output, OutputFormatWide); err != nil {
		return err
	}

	if o.stateFilter != "" {
		o.stateFilter = strings.ToUpper(o.stateFilter)
		if !slices.Contains(validThreadStates, o.stateFilter) {
			return fmt.Errorf("invalid thread state '%s'\nValid states: %v", o.stateFilter, validThreadStates)
		}
	}

	return nil
}

func (o *threaddumpCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

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
		fmt.Println("No threads match the specified filters.")
		return nil
	}

	if len(filteredThreads) < len(threaddump.Threads) {
		fmt.Printf("Showing %d filtered threads:\n\n", len(filteredThreads))
	}

	maxFrames := defaultMaxStackFrames
	if o.wideMode {
		maxFrames = -1
	}

	for i, thread := range filteredThreads {
		displayThread(thread, i+1, o.wideMode, o.noStacktrace, maxFrames)
	}

	return nil
}

func (o *threaddumpCommandOperations) filterThreads(threads []actuator.Thread) ([]actuator.Thread, map[string]int) {
	var filtered []actuator.Thread
	stateCounts := make(map[string]int)

	for _, thread := range threads {
		stateCounts[thread.ThreadState]++

		if o.stateFilter != "" && !strings.EqualFold(thread.ThreadState, o.stateFilter) {
			continue
		}
		if o.nameFilter != "" && !strings.Contains(strings.ToLower(thread.ThreadName), strings.ToLower(o.nameFilter)) {
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

func displayThread(thread actuator.Thread, index int, wideMode, noStacktrace bool, maxFrames int) {
	fmt.Printf("Thread #%d: %s (ID: %d)\n", index, thread.ThreadName, thread.ThreadID)
	fmt.Printf("  State: %s\n", thread.ThreadState)
	fmt.Printf("  Daemon: %t, In Native: %t, Suspended: %t\n", thread.Daemon, thread.InNative, thread.Suspended)

	if thread.Priority > 0 && wideMode {
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

	if thread.LockOwnerId > 0 {
		fmt.Printf("  Waiting on lock owned by thread ID: %d\n", thread.LockOwnerId)
	}

	if !noStacktrace && len(thread.StackTrace) > 0 {
		displayStackTrace(thread.StackTrace, maxFrames)
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
