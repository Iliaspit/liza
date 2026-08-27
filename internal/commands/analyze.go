package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/liza-mas/liza/internal/brand"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// AnalyzeCommand runs circuit breaker analysis and prints the result to stdout.
// Delegates business logic to ops.Analyze.
func AnalyzeCommand(projectRoot string) error {
	result, err := ops.Analyze(projectRoot)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	writeAnalyzeResult(os.Stdout, result)
	return nil
}

func writeAnalyzeResult(w io.Writer, result *ops.AnalyzeResult) {
	if result.Pattern == "" {
		fmt.Fprintln(w, "Circuit breaker: OK — no patterns detected")
		return
	}

	if result.Response == models.CircuitBreakerResponseHalt {
		fmt.Fprintln(w, "🚨 CIRCUIT BREAKER TRIGGERED — HALT")
	} else {
		fmt.Fprintf(w, "Circuit breaker response: %s\n", result.Response)
	}
	fmt.Fprintf(w, "Pattern: %s\n", result.Pattern)
	fmt.Fprintf(w, "Severity: %s\n", result.Severity)
	fmt.Fprintf(w, "Response: %s\n", result.Response)
	fmt.Fprintf(w, "Evidence class: %s\n", result.Classification)
	fmt.Fprintf(w, "Evidence: %s\n", result.Evidence)
	fmt.Fprintf(w, "Explanation: %s\n", result.Explanation)

	switch result.Response {
	case models.CircuitBreakerResponseWarning:
		fmt.Fprintln(w, "State action: none — this evidence was already acknowledged")
	case models.CircuitBreakerResponseCheckpoint:
		fmt.Fprintln(w, "State action: sprint moved to CHECKPOINT")
		fmt.Fprintf(w, "Recovery: run `%s` to continue\n", brand.Command("resume"))
	case models.CircuitBreakerResponseHalt:
		fmt.Fprintln(w, "State action: execution halted")
		fmt.Fprintf(w, "Recovery: run `%s` after remediation\n", brand.Command("resume"))
	}
	if result.ReportPath != "" {
		fmt.Fprintf(w, "\nReport written to: %s\n", result.ReportPath)
	}
}
