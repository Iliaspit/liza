// Package acp contains an independent plugin package for ACP session
// management and run benchmarking.
//
// It is intentionally protocol-light: it captures ACP-style events,
// tracks per-task warm session continuity, and computes simple comparison
// metrics to support testable confidence before wiring into Liza runtime.
package acp
