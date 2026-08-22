package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/jsonout"
	"github.com/liza-mas/liza/internal/ops"
)

func TestRetargetDependency_VerboseDiagnostic(t *testing.T) {
	const sentinel = "retarget-underlying-error-secret"
	details := map[string]any{
		"operation":         "retarget-dependency",
		"task_id":           "A",
		"old_dependency":    "old-dep",
		"new_dependencies":  []string{"B"},
		"phase":             "candidate-state-validation",
		"cycle_path":        []string{"A", "B", "C", "A"},
		"diagnostic_action": "retarget_dependency_rejected",
	}
	opErr := &ops.OperationalError{
		Code:    "validation",
		Phase:   "candidate-state-validation",
		Message: "retarget dependency rejected because the candidate state contains a dependency cycle",
		Details: details,
		Err:     fmt.Errorf("sentinel underlying failure: %s", sentinel),
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	writeRetargetDependencyVerboseDiagnostic(&stderr, opErr)
	err := jsonout.WriteResult(&stdout, nil, nil, opErr)
	if !errors.Is(err, jsonout.ErrAlreadyWritten) {
		t.Fatalf("WriteResult() error = %v, want ErrAlreadyWritten", err)
	}

	var envelope jsonout.Envelope
	stdoutDecoder := json.NewDecoder(&stdout)
	if err := stdoutDecoder.Decode(&envelope); err != nil {
		t.Fatalf("decode stdout envelope: %v", err)
	}
	if err := assertJSONStreamEOF(stdoutDecoder); err != nil {
		t.Fatalf("stdout contains output beyond one JSON envelope: %v", err)
	}
	if envelope.OK || envelope.Error == nil {
		t.Fatalf("stdout envelope = %#v, want one error envelope", envelope)
	}
	if envelope.Error.Code != "validation" || envelope.Error.Message != opErr.Message {
		t.Fatalf("stdout error = %#v, want classified validation message %q", envelope.Error, opErr.Message)
	}

	var diagnostic map[string]any
	stderrDecoder := json.NewDecoder(&stderr)
	if err := stderrDecoder.Decode(&diagnostic); err != nil {
		t.Fatalf("decode stderr diagnostic: %v", err)
	}
	if err := assertJSONStreamEOF(stderrDecoder); err != nil {
		t.Fatalf("stderr contains output beyond one safe diagnostic: %v", err)
	}
	if len(diagnostic) != 2 {
		t.Fatalf("stderr diagnostic keys = %#v, want only message and details", diagnostic)
	}
	if diagnostic["message"] != envelope.Error.Message {
		t.Fatalf("stderr diagnostic message = %q, want stdout classified message %q", diagnostic["message"], envelope.Error.Message)
	}
	if !reflect.DeepEqual(diagnostic["details"], envelope.Error.Details) {
		t.Fatalf("stderr details = %#v, want stdout safe details %#v", diagnostic["details"], envelope.Error.Details)
	}
	if strings.Contains(stdout.String(), sentinel) || strings.Contains(stderr.String(), sentinel) {
		t.Fatalf("JSON verbose output leaked underlying sentinel: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func assertJSONStreamEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected extra JSON value: %#v", extra)
}
