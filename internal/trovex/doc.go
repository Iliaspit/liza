// Package trovex owns Liza's optional Trovex documentation search activation
// and command-planning contracts.
//
// Runtime lifecycle callers use RuntimeEnabled, RefreshIndex, and
// BuildPromptMetadata to guard Trovex integration with LIZA_ENABLE_TROVEX,
// refresh the markdown documentation index for a target root, and produce
// prompt-safe metadata for agent guidance. Trovex stores its index in
// ~/.trovex-data/trovex.db; no project-local artifacts are created. When the
// operator has not configured an explicit embedding model or OpenAI API key,
// command plans default to the local fastembed model (BAAI/bge-small-en-v1.5)
// so indexing works offline without external API credentials.
package trovex
