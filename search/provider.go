//go:build ignore
// +build ignore

package search

import (
	"xdcc-tui/xdcc"
)

type SearchResult struct {
	Name string
	Size int64
	URL  xdcc.IRCFile
}

type Provider interface {
	Search(keywords []string) ([]SearchResult, error)
}

// ProviderAggregator combines multiple search providers
type ProviderAggregator struct {
	providers []Provider
}

func NewProviderAggregator(providers ...Provider) *ProviderAggregator {
	return &ProviderAggregator{
		providers: providers,
	}
}

func (a *ProviderAggregator) Search(keywords []string) ([]SearchResult, error) {
	var results []SearchResult
	for _, provider := range a.providers {
		providerResults, err := provider.Search(keywords)
		if err != nil {
			continue // Skip failed providers
		}
		results = append(results, providerResults...)
	}
	return results, nil
}
