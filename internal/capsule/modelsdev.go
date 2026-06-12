package capsule

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const ModelsDevAPIURL = "https://models.dev/api.json"

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func FetchModelsDevProvider(ctx context.Context, client HTTPDoer, providerID string) (ProviderMetadata, error) {
	if providerID == "" {
		return ProviderMetadata{}, fmt.Errorf("models.dev provider ID is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsDevAPIURL, nil)
	if err != nil {
		return ProviderMetadata{}, err
	}
	req.Header.Set("User-Agent", "liza-capsule/1")
	resp, err := client.Do(req)
	if err != nil {
		return ProviderMetadata{}, fmt.Errorf("fetch models.dev catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ProviderMetadata{}, fmt.Errorf("fetch models.dev catalog: HTTP %d", resp.StatusCode)
	}

	var catalog map[string]modelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return ProviderMetadata{}, fmt.Errorf("parse models.dev catalog: %w", err)
	}
	raw, ok := catalog[providerID]
	if !ok {
		return ProviderMetadata{}, fmt.Errorf("models.dev provider %q not found", providerID)
	}
	return raw.toProviderMetadata(providerID), nil
}

type modelsDevProvider struct {
	ID       string                    `json:"id"`
	Name     string                    `json:"name"`
	API      string                    `json:"api"`
	BaseURL  string                    `json:"baseURL"`
	BaseURL2 string                    `json:"base_url"`
	Models   map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (p modelsDevProvider) toProviderMetadata(providerID string) ProviderMetadata {
	baseURL := p.API
	if baseURL == "" {
		baseURL = p.BaseURL
	}
	if baseURL == "" {
		baseURL = p.BaseURL2
	}
	name := p.Name
	if name == "" {
		name = providerID
	}
	models := make([]ModelMetadata, 0, len(p.Models))
	for id, model := range p.Models {
		modelID := model.ID
		if modelID == "" {
			modelID = id
		}
		models = append(models, ModelMetadata{ID: modelID, Name: model.Name})
	}
	return ProviderMetadata{
		ID:      providerID,
		Name:    name,
		BaseURL: baseURL,
		Models:  models,
	}
}
