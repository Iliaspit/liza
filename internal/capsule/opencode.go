package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type ProviderMetadata struct {
	ID      string
	Name    string
	BaseURL string
	Models  []ModelMetadata
}

type ModelMetadata struct {
	ID   string
	Name string
}

func PresetFromProviderMetadata(source string, provider ProviderMetadata, apiKeyEnv string, preferredModels []string) (OpenCodePreset, error) {
	if provider.ID == "" {
		return OpenCodePreset{}, fmt.Errorf("provider metadata missing ID")
	}
	if provider.BaseURL == "" {
		return OpenCodePreset{}, fmt.Errorf("provider %q metadata missing base URL", provider.ID)
	}
	if apiKeyEnv == "" {
		return OpenCodePreset{}, fmt.Errorf("provider %q missing API key environment variable", provider.ID)
	}
	models := make(map[string]string, len(provider.Models))
	for _, model := range provider.Models {
		if model.ID == "" {
			continue
		}
		name := model.Name
		if name == "" {
			name = model.ID
		}
		models[model.ID] = name
	}
	if len(models) == 0 {
		return OpenCodePreset{}, fmt.Errorf("provider %q metadata contains no models", provider.ID)
	}

	selected := firstAvailableModel(models, preferredModels)
	if selected == "" {
		selected = sortedModelIDs(models)[0]
	}
	small := ""
	for _, candidate := range []string{"gpt-oss-20b", "gpt-4.1-mini", "claude-haiku-4-5"} {
		if _, ok := models[candidate]; ok && candidate != selected {
			small = candidate
			break
		}
	}
	return OpenCodePreset{
		ID:               source + "-" + provider.ID,
		ProviderID:       provider.ID,
		ProviderName:     provider.Name,
		BaseURL:          provider.BaseURL,
		APIKeyEnv:        apiKeyEnv,
		Model:            selected,
		SmallModel:       small,
		Models:           models,
		EnabledProviders: []string{provider.ID},
		Source:           source,
	}, nil
}

func DefaultOpenCodePreset(id string) (OpenCodePreset, error) {
	if id == "" {
		id = "openai-compatible"
	}
	switch id {
	case "openai-compatible", "generic":
		return OpenCodePreset{
			ID:           id,
			ProviderID:   "capsule",
			ProviderName: "Capsule Provider",
			BaseURL:      "https://api.openai.com/v1",
			APIKeyEnv:    "CAPSULE_PROVIDER_API_KEY",
			Model:        "gpt-oss-120b",
			SmallModel:   "gpt-oss-20b",
			Models: map[string]string{
				"gpt-oss-120b": "GPT OSS 120B",
				"gpt-oss-20b":  "GPT OSS 20B",
				"grok-code":    "Grok Code",
			},
			EnabledProviders: []string{"capsule"},
			Source:           "custom-openai-compatible",
		}, nil
	default:
		return OpenCodePreset{}, fmt.Errorf("unknown capsule preset %q", id)
	}
}

func firstAvailableModel(models map[string]string, preferred []string) string {
	for _, model := range preferred {
		if _, ok := models[model]; ok {
			return model
		}
	}
	return ""
}

func sortedModelIDs(models map[string]string) []string {
	keys := make([]string, 0, len(models))
	for model := range models {
		keys = append(keys, model)
	}
	sort.Strings(keys)
	return keys
}

func RenderOpenCodeConfig(preset OpenCodePreset) ([]byte, error) {
	models := make(map[string]map[string]string, len(preset.Models))
	keys := make([]string, 0, len(preset.Models))
	for model := range preset.Models {
		keys = append(keys, model)
	}
	sort.Strings(keys)
	for _, model := range keys {
		models[model] = map[string]string{"name": preset.Models[model]}
	}

	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   preset.Model,
		"provider": map[string]any{
			preset.ProviderID: map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": preset.ProviderName,
				"options": map[string]string{
					"baseURL": preset.BaseURL,
					"apiKey":  "{env:" + preset.APIKeyEnv + "}",
				},
				"models": models,
			},
		},
	}
	if preset.SmallModel != "" {
		config["small_model"] = preset.SmallModel
	}
	if len(preset.EnabledProviders) > 0 {
		config["enabled_providers"] = preset.EnabledProviders
	}
	return json.MarshalIndent(config, "", "  ")
}

func WriteOpenCodeConfig(dir string, preset OpenCodePreset) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create OpenCode config dir: %w", err)
	}
	data, err := RenderOpenCodeConfig(preset)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return "", fmt.Errorf("failed to write OpenCode config: %w", err)
	}
	return path, nil
}
