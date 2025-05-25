package search

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"
	"xdcc-tui/xdcc"
)

type XdccFileInfo struct {
	URL  xdcc.IRCFile
	Name string
	Size int64
	Slot int
}

type XdccSearchProvider interface {
	Search(keywords []string) ([]XdccFileInfo, error)
}

type ProviderAggregator struct {
	providerList []XdccSearchProvider
}

const MaxProviders = 100

func NewProviderAggregator(providers ...XdccSearchProvider) *ProviderAggregator {
	return &ProviderAggregator{
		providerList: providers,
	}
}

func (registry *ProviderAggregator) AddProvider(provider XdccSearchProvider) {
	registry.providerList = append(registry.providerList, provider)
}

const MaxResults = 1024 // Maximum number of results that can be returned

func (registry *ProviderAggregator) Search(keywords []string) ([]XdccFileInfo, error) {
	// Use real search data
	if len(registry.providerList) == 0 {
		return []XdccFileInfo{}, nil
	}
	
	allResults := make(map[xdcc.IRCFile]XdccFileInfo)

	mtx := sync.Mutex{}
	errChan := make(chan error, len(registry.providerList))
	
	// Use a timeout to prevent hanging indefinitely
	timeoutChan := time.After(10 * time.Second)
	doneChan := make(chan struct{})
	
	wg := sync.WaitGroup{}
	wg.Add(len(registry.providerList))
	for _, p := range registry.providerList {
		go func(p XdccSearchProvider) {
			defer wg.Done()
			
			resList, err := p.Search(keywords)
			if err != nil {
				errChan <- err
				return
			}

			mtx.Lock()
			for _, res := range resList {
				allResults[res.URL] = res
			}
			mtx.Unlock()
		}(p)
	}
	
	// Wait for all goroutines to complete or timeout
	go func() {
		wg.Wait()
		close(doneChan)
	}()
	
	// Wait for either completion or timeout
	select {
	case <-doneChan:
		// All providers completed successfully
	case <-timeoutChan:
		// Search timed out, but we'll return what we have so far
	}

	results := make([]XdccFileInfo, 0, MaxResults)
	for _, res := range allResults {
		results = append(results, res)
	}
	
	// Sort results by file size (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Size > results[j].Size
	})
	
	return results, nil
}

const (
	KiloByte = 1024
	MegaByte = KiloByte * 1024
	GigaByte = MegaByte * 1024
)

// createMockResults removed to use real search data

func parseFileSize(sizeStr string) (int64, error) {
	if len(sizeStr) == 0 {
		return -1, errors.New("empty string")
	}
	lastChar := sizeStr[len(sizeStr)-1]
	sizePart := sizeStr[:len(sizeStr)-1]

	size, err := strconv.ParseFloat(sizePart, 32)

	if err != nil {
		return -1, err
	}
	switch lastChar {
	case 'G':
		return int64(size * GigaByte), nil
	case 'M':
		return int64(size * MegaByte), nil
	case 'K':
		return int64(size * KiloByte), nil
	}
	return -1, errors.New("unable to parse: " + sizeStr)
}
