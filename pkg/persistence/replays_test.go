package persistence

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestReplayCompressionRoundTrip(t *testing.T) {
	raw := bytes.Repeat([]byte(`{"matchId":"match","players":{"u":{"score":5000}}}`), 100)
	compressed, sum, err := compressReplay(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) >= len(raw) {
		t.Fatalf("compressed replay size=%d, raw=%d", len(compressed), len(raw))
	}
	decoded, err := decompressReplay(compressed, replayCodecZstd, len(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("decoded replay differs from original")
	}
	if len(sum) != 32 {
		t.Fatalf("sha256 length=%d", len(sum))
	}
}

func TestReplayDecompressionRejectsInvalidMetadata(t *testing.T) {
	if _, err := decompressReplay([]byte("bad"), 99, 3); err == nil {
		t.Fatal("expected unsupported codec error")
	}
	if _, err := decompressReplay([]byte("bad"), replayCodecZstd, maxReplayDecodedBytes+1); err == nil {
		t.Fatal("expected decoded size limit error")
	}
}

func TestCompactLocationEncoding(t *testing.T) {
	row := compactMapRow(12.3456789, -98.7654321)
	if row.LatE7 != 123456789 || row.LngE7 != -987654321 {
		t.Fatalf("unexpected compact coordinates: %d %d", row.LatE7, row.LngE7)
	}
	if row.RandKey < 0 || row.RandKey > 16_777_215 {
		t.Fatalf("rand key out of range: %d", row.RandKey)
	}
	if got := compactAngle(359.99, false); got != -1 {
		t.Fatalf("heading encoding=%d, want -1", got)
	}
	if got := compactAngle(120, true); got != 9000 {
		t.Fatalf("pitch encoding=%d, want 9000", got)
	}
}

func TestReplayReadIntegration(t *testing.T) {
	dsn := os.Getenv("REPLAY_TEST_POSTGRES_URL")
	matchID := os.Getenv("REPLAY_TEST_MATCH_ID")
	if dsn == "" || matchID == "" {
		t.Skip("REPLAY_TEST_POSTGRES_URL and REPLAY_TEST_MATCH_ID are required")
	}
	t.Setenv("POSTGRES_URL", dsn)
	store, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	raw, found, err := store.GetFinalMatchSnapshot(matchID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("compressed replay not found")
	}
	if !json.Valid(raw) {
		t.Fatal("decoded replay is not valid JSON")
	}
}
