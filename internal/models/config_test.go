package models

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIsTDDEnabled_NilDefaultsTrue(t *testing.T) {
	c := Config{}
	if !c.IsTDDEnabled() {
		t.Error("IsTDDEnabled() = false for nil TDDEnabled, want true")
	}
}

func TestIsTDDEnabled_ExplicitTrue(t *testing.T) {
	v := true
	c := Config{TDDEnabled: &v}
	if !c.IsTDDEnabled() {
		t.Error("IsTDDEnabled() = false for explicit true, want true")
	}
}

func TestIsTDDEnabled_ExplicitFalse(t *testing.T) {
	v := false
	c := Config{TDDEnabled: &v}
	if c.IsTDDEnabled() {
		t.Error("IsTDDEnabled() = true for explicit false, want false")
	}
}

func TestTDDEnabled_YAMLRoundTrip_False(t *testing.T) {
	v := false
	original := Config{
		MaxCoderIterations: 10,
		TDDEnabled:         &v,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Verify tdd_enabled: false is present in YAML
	if !strings.Contains(string(data), "tdd_enabled: false") {
		t.Errorf("YAML does not contain 'tdd_enabled: false':\n%s", data)
	}

	var restored Config
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.TDDEnabled == nil {
		t.Fatal("TDDEnabled is nil after round-trip, want *false")
	}
	if *restored.TDDEnabled {
		t.Error("TDDEnabled is true after round-trip, want false")
	}
}

func TestTDDEnabled_YAMLRoundTrip_Nil(t *testing.T) {
	original := Config{
		MaxCoderIterations: 10,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Verify tdd_enabled is NOT present in YAML (omitempty with nil)
	if strings.Contains(string(data), "tdd_enabled") {
		t.Errorf("YAML should not contain 'tdd_enabled' for nil value:\n%s", data)
	}

	var restored Config
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.TDDEnabled != nil {
		t.Errorf("TDDEnabled = %v after round-trip, want nil", *restored.TDDEnabled)
	}
}
