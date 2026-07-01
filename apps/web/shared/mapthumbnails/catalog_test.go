package mapthumbnails

import "testing"

func TestValidKeyUsesEmbeddedCatalog(t *testing.T) {
	if !ValidKey("generic/variant-1") {
		t.Fatal("expected generated generic thumbnail key to be valid")
	}
	if !ValidKey("continents/africa") {
		t.Fatal("expected generated continent thumbnail key to be valid")
	}
	if !ValidKey("countries/france") {
		t.Fatal("expected generated country thumbnail key to be valid")
	}
	if ValidKey("countries/does-not-exist") {
		t.Fatal("unexpected arbitrary country thumbnail key to be valid")
	}
	if ValidKey("continents/not-real") {
		t.Fatal("unexpected arbitrary continent thumbnail key to be valid")
	}
}
