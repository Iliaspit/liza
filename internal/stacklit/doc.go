// Package stacklit owns the optional Stacklit runtime indexing contract.
//
// Runtime lifecycle callers use RuntimeEnabled, RefreshIndex, and
// AvailableIndexes to guard Stacklit generation with the branded
// ENABLE_STACKLIT variable,
// generate a target-local stacklit.json, and expose only existing absolute index
// paths for prompt guidance. stacklit.json may be tracked or ignored; task
// worktree generation rejects only the unsafe middle state where it is neither.
// The runtime does not create or mutate stacklit-insights.json or .stacklitrc.json;
// Stacklit consumes those committed operator-curated files when they exist in
// the target tree.
package stacklit
