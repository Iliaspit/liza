// Package stacklit owns Liza's optional Stacklit runtime indexing contract.
//
// Runtime lifecycle callers use RuntimeEnabled, RefreshIndex, and
// AvailableIndexes to guard Stacklit generation with LIZA_ENABLE_STACKLIT,
// generate a target-local stacklit.json, and expose only existing absolute index
// paths for prompt guidance. Liza does not create or mutate stacklit-insights.json
// or .stacklitrc.json; Stacklit consumes those committed operator-curated files
// when they exist in the target tree.
package stacklit
