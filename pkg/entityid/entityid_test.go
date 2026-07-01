package entityid

import (
	"strings"
	"testing"
)

func TestNewUUIDv7RoundTrip(t *testing.T) {
	id := New()
	if len(id) != 36 || id[14] != '7' {
		t.Fatalf("New() = %q, want UUIDv7", id)
	}
	public, err := Public(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 26 {
		t.Fatalf("Public() length = %d, want 26", len(public))
	}
	parsed, err := Parse(strings.ToLower(public))
	if err != nil {
		t.Fatal(err)
	}
	if parsed != id {
		t.Fatalf("round trip = %q, want %q", parsed, id)
	}
}

func TestPublicEncodingCompatibility(t *testing.T) {
	const id = "01976ad2-9c42-7d31-a820-63d7c5279841"
	public, err := Public(id)
	if err != nil {
		t.Fatal(err)
	}
	if public != "01JXND5722FMRTG833TZ2JF621" {
		t.Fatalf("Public() = %q", public)
	}
}

func TestParseCrockfordAliases(t *testing.T) {
	id := "00000000-0000-7000-8000-000000000001"
	public, err := Public(id)
	if err != nil {
		t.Fatal(err)
	}
	aliased := strings.NewReplacer("0", "O", "1", "L").Replace(public)
	parsed, err := Parse(aliased)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != id {
		t.Fatalf("Parse() = %q, want %q", parsed, id)
	}
}

func TestDeriveIsStableUUID(t *testing.T) {
	first := Derive("conversation", "party:legacy")
	second := Derive("conversation", "party:legacy")
	if first != second || first[14] != '3' {
		t.Fatalf("Derive() = %q and %q", first, second)
	}
}

func TestDeriveMatchesMigrationFunction(t *testing.T) {
	if got := Derive("user", "u_legacy"); got != "fd5a577b-d281-315e-8ccc-0c5a4c7d60b0" {
		t.Fatalf("Derive() = %q", got)
	}
}
