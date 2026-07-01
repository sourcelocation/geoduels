package persistence

import "testing"

func TestNormalizeThumbnailKeyUsesGeneratedCatalog(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		variant  int
		expected string
	}{
		{name: "catalog country", key: "countries/france", variant: 2, expected: "countries/france"},
		{name: "normalizes catalog key", key: " /CONTINENTS/AFRICA/ ", variant: 2, expected: "continents/africa"},
		{name: "rejects arbitrary country", key: "countries/does-not-exist", variant: 3, expected: "generic/variant-3"},
		{name: "rejects arbitrary continent", key: "continents/not-real", variant: 4, expected: "generic/variant-4"},
		{name: "normalizes invalid variant", key: "invalid", variant: 99, expected: "generic/variant-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := normalizeThumbnailKey(test.key, test.variant); actual != test.expected {
				t.Fatalf("normalizeThumbnailKey(%q, %d) = %q, want %q", test.key, test.variant, actual, test.expected)
			}
		})
	}
}
