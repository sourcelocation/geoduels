package persistence

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

const (
	replayCodecZstd       = 1
	replaySchemaVersion   = 1
	replayRetentionDays   = 30
	maxReplayDecodedBytes = 16 << 20
)

func compressReplay(raw []byte) ([]byte, [sha256.Size]byte, error) {
	sum := sha256.Sum256(raw)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return nil, sum, err
	}
	defer encoder.Close()
	return encoder.EncodeAll(raw, make([]byte, 0, len(raw)/3)), sum, nil
}

func decompressReplay(compressed []byte, codec int, uncompressedBytes int) ([]byte, error) {
	if codec != replayCodecZstd {
		return nil, fmt.Errorf("unsupported replay codec %d", codec)
	}
	if uncompressedBytes < 0 || uncompressedBytes > maxReplayDecodedBytes {
		return nil, errors.New("replay exceeds decoded size limit")
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxReplayDecodedBytes))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	raw, err := decoder.DecodeAll(compressed, make([]byte, 0, uncompressedBytes))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxReplayDecodedBytes {
		return nil, errors.New("replay exceeds decoded size limit")
	}
	if uncompressedBytes > 0 && len(raw) != uncompressedBytes {
		return nil, errors.New("replay decoded size mismatch")
	}
	return raw, nil
}
