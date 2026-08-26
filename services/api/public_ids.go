package main

import (
	"strings"

	"geoduels/pkg/entityid"
)

func (a *api) resolveEntityID(_ string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if id, err := entityid.Parse(value); err == nil {
		return id
	}
	return ""
}

func resolveCompactEntityID(value string) string {
	value = strings.TrimSpace(value)
	if id, err := entityid.Parse(value); err == nil {
		return id
	}
	return value
}
