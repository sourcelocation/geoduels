package main

import (
	"strings"

	"geoduels/pkg/entityid"
)

type legacyEntityIDResolver interface {
	ResolveLegacyEntityID(entityType, legacyID string) (string, bool, error)
}

func (a *api) resolveEntityID(entityType, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if id, err := entityid.Parse(value); err == nil {
		return id
	}
	if resolver, ok := a.store.(legacyEntityIDResolver); ok {
		if id, found, err := resolver.ResolveLegacyEntityID(entityType, value); err == nil && found {
			return id
		}
	}
	return value
}

func resolveCompactEntityID(value string) string {
	value = strings.TrimSpace(value)
	if id, err := entityid.Parse(value); err == nil {
		return id
	}
	return value
}
