package ops

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/analysis"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

type analyzeTestHooks struct {
	beforeModify func()
}

var testAnalyzeHooks *analyzeTestHooks

// AnalyzeResult contains the outcome of a circuit breaker analysis.
type AnalyzeResult struct {
	Triggered      bool                               `json:"triggered"`
	Pattern        string                             `json:"pattern"`
	Severity       string                             `json:"severity"`
	Evidence       string                             `json:"evidence"`
	Response       models.CircuitBreakerResponseType  `json:"response"`
	Classification models.CircuitBreakerEvidenceClass `json:"classification"`
	Explanation    string                             `json:"explanation"`
	ReportPath     string                             `json:"report_path"`
}

// Analyze detects circuit breaker patterns from blackboard anomalies. Generates
// a report and transitions system mode to CIRCUIT_BREAKER_TRIPPED if triggered.
// No terminal I/O.
func Analyze(projectRoot string) (*AnalyzeResult, error) {
	lizaPaths := paths.New(projectRoot)
	statePath := lizaPaths.StatePath()
	reportPath := lizaPaths.CircuitBreakerReportPath()

	blackboard := db.For(statePath)

	if _, err := blackboard.Read(); err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}
	if testAnalyzeHooks != nil && testAnalyzeHooks.beforeModify != nil {
		testAnalyzeHooks.beforeModify()
	}

	var committed analysis.PatternResult
	var committedReportPath string
	var timestamp time.Time
	err := blackboard.Modify(func(s *models.State) error {
		timestamp = time.Now()
		var consideredAnomalies []models.Anomaly
		var suppressedCount int
		committed, consideredAnomalies, suppressedCount = analysis.DetectUnacknowledgedPatterns(s)
		s.CircuitBreaker.LastCheck = timestamp

		activeResponse := s.CircuitBreaker.CurrentResponse
		if activeResponse != nil && activeResponse.Response == models.CircuitBreakerResponseHalt {
			committed = patternResultFromResponse(activeResponse)
			committedReportPath = activeResponse.ReportFile
			return nil
		}
		if activeResponse != nil && activeResponse.Pattern == "provider_audit_degradation" && activeResponse.Response == models.CircuitBreakerResponseCheckpoint {
			if committed.Response != models.CircuitBreakerResponseHalt {
				committed = patternResultFromResponse(activeResponse)
				committedReportPath = activeResponse.ReportFile
				return nil
			}
			if err := supersedeActiveProviderCheckpoint(s, activeResponse); err != nil {
				return err
			}
		}

		if committed.Pattern == "" {
			s.CircuitBreaker.Status = "OK"
			s.CircuitBreaker.CurrentTrigger = nil
			if s.Config.Mode == models.SystemModeCircuitBreakerTripped {
				s.Config.Mode = models.SystemModeRunning
			}
			s.CircuitBreaker.History = append(s.CircuitBreaker.History, models.CircuitBreakerHistory{
				Timestamp: timestamp,
				Result:    "OK",
			})
			return nil
		}

		report := analysis.GenerateReport(committed, consideredAnomalies, timestamp, suppressedCount)
		if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		committedReportPath = reportPath

		applyCircuitBreakerResponse(s, committed, timestamp, reportPath)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update circuit breaker state: %w", err)
	}

	result := &AnalyzeResult{
		Triggered:      committed.Triggered,
		Pattern:        committed.Pattern,
		Severity:       committed.Severity,
		Evidence:       committed.Evidence,
		Response:       committed.Response,
		Classification: committed.Classification,
		Explanation:    committed.Explanation,
	}
	if committed.Pattern != "" {
		result.ReportPath = committedReportPath
	}
	return result, nil
}

func patternResultFromResponse(response *models.CircuitBreakerResponse) analysis.PatternResult {
	return analysis.PatternResult{
		Triggered:      response.Response == models.CircuitBreakerResponseHalt,
		Pattern:        response.Pattern,
		Severity:       response.Severity,
		Response:       response.Response,
		Classification: response.Classification,
		Explanation:    response.Explanation,
	}
}

func supersedeActiveProviderCheckpoint(state *models.State, response *models.CircuitBreakerResponse) error {
	for i := len(state.CircuitBreaker.History) - 1; i >= 0; i-- {
		entry := &state.CircuitBreaker.History[i]
		if !entry.Timestamp.Equal(response.Timestamp) || entry.Response != response.Response || entry.Pattern == nil || *entry.Pattern != response.Pattern {
			continue
		}
		if entry.Resolution != nil || entry.ResolvedAt != nil {
			return fmt.Errorf("active provider checkpoint history boundary is already acknowledged")
		}
		if entry.SupersededByResponse != "" {
			return fmt.Errorf("active provider checkpoint history boundary is already superseded")
		}
		entry.SupersededByResponse = models.CircuitBreakerResponseHalt
		return nil
	}

	return fmt.Errorf("active provider checkpoint has no matching history boundary")
}

func applyCircuitBreakerResponse(state *models.State, result analysis.PatternResult, timestamp time.Time, reportPath string) {
	pattern := result.Pattern
	severity := result.Severity
	historyResult := string(result.Response)

	switch result.Response {
	case models.CircuitBreakerResponseWarning:
		// Historical evidence is observation-only; preserve all active state.
	case models.CircuitBreakerResponseCheckpoint:
		state.CircuitBreaker.Status = "OK"
		state.CircuitBreaker.CurrentTrigger = nil
		state.Sprint.Status = models.SprintStatusCheckpoint
		state.CircuitBreaker.CurrentResponse = circuitBreakerResponse(result, timestamp, reportPath)
	case models.CircuitBreakerResponseHalt:
		historyResult = "TRIGGERED"
		state.CircuitBreaker.Status = "TRIGGERED"
		state.CircuitBreaker.CurrentTrigger = &models.CircuitBreakerTrigger{
			Timestamp:  timestamp,
			Pattern:    result.Pattern,
			Severity:   result.Severity,
			ReportFile: reportPath,
		}
		state.CircuitBreaker.CurrentResponse = circuitBreakerResponse(result, timestamp, reportPath)
		state.Config.Mode = models.SystemModeCircuitBreakerTripped
	}

	state.CircuitBreaker.History = append(state.CircuitBreaker.History, models.CircuitBreakerHistory{
		Timestamp:      timestamp,
		Pattern:        &pattern,
		Severity:       &severity,
		Result:         historyResult,
		Response:       result.Response,
		Classification: result.Classification,
		Explanation:    result.Explanation,
	})
}

func circuitBreakerResponse(result analysis.PatternResult, timestamp time.Time, reportPath string) *models.CircuitBreakerResponse {
	return &models.CircuitBreakerResponse{
		Timestamp:      timestamp,
		Pattern:        result.Pattern,
		Severity:       result.Severity,
		Response:       result.Response,
		Classification: result.Classification,
		Explanation:    result.Explanation,
		ReportFile:     reportPath,
	}
}
