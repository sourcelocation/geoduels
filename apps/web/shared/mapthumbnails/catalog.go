package mapthumbnails

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

type Thumbnail struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Code     string `json:"code,omitempty"`
	Search   string `json:"search"`
}

type Catalog struct {
	Version    int         `json:"version"`
	Thumbnails []Thumbnail `json:"thumbnails"`
}

//go:embed catalog.generated.json
var catalogJSON []byte

var validKeys = loadValidKeys()

func ValidKey(key string) bool {
	_, ok := validKeys[key]
	return ok
}

func loadValidKeys() map[string]struct{} {
	var catalog Catalog
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic(fmt.Sprintf("decode map thumbnail catalog: %v", err))
	}
	if catalog.Version != 1 {
		panic(fmt.Sprintf("unsupported map thumbnail catalog version: %d", catalog.Version))
	}
	if len(catalog.Thumbnails) == 0 {
		panic("map thumbnail catalog is empty")
	}

	keys := make(map[string]struct{}, len(catalog.Thumbnails))
	for _, thumbnail := range catalog.Thumbnails {
		if thumbnail.Key == "" {
			panic("map thumbnail catalog contains an empty key")
		}
		if _, exists := keys[thumbnail.Key]; exists {
			panic(fmt.Sprintf("map thumbnail catalog contains duplicate key %q", thumbnail.Key))
		}
		keys[thumbnail.Key] = struct{}{}
	}
	return keys
}
