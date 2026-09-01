// Package semble owns optional Semble activation and command-planning
// contracts.
//
// Runtime lifecycle callers use RuntimeEnabled and PlanCommands to keep
// the branded ENABLE_SEMBLE strict opt-in, detect the external Semble CLI through a
// fakeable lookup seam, and obtain fixed prewarm/offline validation command
// plans without executing subprocesses. DefaultIgnorePatterns is the ordered
// source of truth for Semble-visible runtime, generated-index, and credential
// exclusions used by later target-safety work.
package semble
