package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/jsonout"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/spf13/cobra"
)

const (
	graphReplanClaimOperation    = "claim-graph-replan"
	graphReplanCompleteOperation = "complete-graph-replan"
)

var requestGraphReplanCmd = &cobra.Command{
	Use:   "request-graph-replan",
	Short: "Request a native orchestrator repair for a proven dependency-graph fault",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) (retErr error) {
		finish := prepareGraphReplanJSON(cmd, &retErr)
		defer finish()
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		runID, _ := cmd.Flags().GetString("run-id")
		requestedBy, _ := cmd.Flags().GetString("requested-by")
		reason, _ := cmd.Flags().GetString("reason")
		result, err := ops.RequestGraphReplan(projectRoot, ops.RequestGraphReplanInput{RunID: runID, RequestedBy: requestedBy, Reason: reason})
		if isJSON(cmd) {
			return jsonout.WriteResult(os.Stdout, result, nil, err)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph re-plan requested: %s (generation %s)\n", result.Request.ID, result.Request.GraphGeneration)
		return nil
	},
}

var claimGraphReplanCmd = &cobra.Command{
	Use:   "claim-graph-replan <request-id>",
	Short: "Bind a graph re-plan request to the current native orchestrator generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (retErr error) {
		finish := prepareGraphReplanJSON(cmd, &retErr)
		defer finish()
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		authority, err := resolveOrchestratorAuthority(cmd)
		if err != nil {
			return err
		}
		resolver, err := loadResolverForRBAC(projectRoot)
		if err != nil {
			return err
		}
		if err := validateAllowedOperation(resolver, authority.ID, graphReplanClaimOperation); err != nil {
			return err
		}
		result, err := ops.ClaimGraphReplan(projectRoot, args[0], authority)
		if isJSON(cmd) {
			return jsonout.WriteResult(os.Stdout, result, nil, err)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph re-plan claimed: %s by %s\n", result.ID, authority.ID)
		return nil
	},
}

var completeGraphReplanCmd = &cobra.Command{
	Use:   "complete-graph-replan <request-id>",
	Short: "Apply an orchestrator-owned graph correction and close it after validation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (retErr error) {
		finish := prepareGraphReplanJSON(cmd, &retErr)
		defer finish()
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		authority, err := resolveOrchestratorAuthority(cmd)
		if err != nil {
			return err
		}
		resolver, err := loadResolverForRBAC(projectRoot)
		if err != nil {
			return err
		}
		if err := validateAllowedOperation(resolver, authority.ID, graphReplanCompleteOperation); err != nil {
			return err
		}
		diagnosis, _ := cmd.Flags().GetString("diagnosis")
		updates, err := graphDependencyUpdatesFromFile(cmd)
		if err != nil {
			return err
		}
		result, err := ops.CompleteGraphReplan(projectRoot, ops.CompleteGraphReplanInput{RequestID: args[0], Diagnosis: diagnosis, Updates: updates}, authority)
		if isJSON(cmd) {
			return jsonout.WriteResult(os.Stdout, result, nil, err)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph re-plan completed: %s (%s)\n", result.Request.ID, result.Request.ResultGeneration)
		return nil
	},
}

func graphDependencyUpdatesFromFile(cmd *cobra.Command) ([]ops.GraphDependencyUpdate, error) {
	path, _ := cmd.Flags().GetString("updates-file")
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cliValidationWrap("reading graph updates file", err)
	}
	var updates []ops.GraphDependencyUpdate
	if err := json.Unmarshal(data, &updates); err != nil {
		return nil, cliValidationWrap("parsing graph updates file", err)
	}
	return updates, nil
}

func prepareGraphReplanJSON(cmd *cobra.Command, retErr *error) func() {
	if !isJSON(cmd) {
		return func() {}
	}
	log.SetOutput(io.Discard)
	return func() {
		log.SetOutput(os.Stderr)
		if *retErr != nil && !errors.Is(*retErr, jsonout.ErrAlreadyWritten) {
			_ = jsonout.WriteResult(os.Stdout, nil, nil, *retErr)
			*retErr = jsonout.ErrAlreadyWritten
		}
	}
}

func init() {
	rootCmd.AddCommand(requestGraphReplanCmd, claimGraphReplanCmd, completeGraphReplanCmd)
	for _, command := range []*cobra.Command{requestGraphReplanCmd, claimGraphReplanCmd, completeGraphReplanCmd} {
		addJSONFlag(command)
	}
	requestGraphReplanCmd.Flags().String("run-id", "", "exact Lisa goal ID for this run (required)")
	requestGraphReplanCmd.Flags().String("requested-by", "", "controller identity recorded in the audit request (required)")
	requestGraphReplanCmd.Flags().String("reason", "", "proven graph-deadlock reason (required)")
	_ = requestGraphReplanCmd.MarkFlagRequired("run-id")
	_ = requestGraphReplanCmd.MarkFlagRequired("requested-by")
	_ = requestGraphReplanCmd.MarkFlagRequired("reason")
	addAgentIDFlag(claimGraphReplanCmd)
	addAgentIDFlag(completeGraphReplanCmd)
	completeGraphReplanCmd.Flags().String("diagnosis", "", "orchestrator diagnosis (required)")
	completeGraphReplanCmd.Flags().String("updates-file", "", "optional JSON array of generation-fenced dependency updates")
	_ = completeGraphReplanCmd.MarkFlagRequired("diagnosis")
}
