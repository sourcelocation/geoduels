package entityid

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	generatorMu     sync.Mutex
	lastMillisecond int64
	lastRandom      [10]byte
)

// New returns a canonical UUIDv7 string. The random tail is incremented when
// multiple identifiers are generated in the same millisecond, preserving
// locality and uniqueness within a process.
func New() string {
	generatorMu.Lock()
	defer generatorMu.Unlock()

	now := time.Now().UnixMilli()
	if now > lastMillisecond {
		lastMillisecond = now
		if _, err := rand.Read(lastRandom[:]); err != nil {
			panic("entityid: secure random source unavailable: " + err.Error())
		}
	} else {
		now = lastMillisecond
		incrementRandom()
	}

	var raw [16]byte
	raw[0] = byte(now >> 40)
	raw[1] = byte(now >> 32)
	raw[2] = byte(now >> 24)
	raw[3] = byte(now >> 16)
	raw[4] = byte(now >> 8)
	raw[5] = byte(now)
	copy(raw[6:], lastRandom[:])
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return formatUUID(raw)
}

// Derive returns a stable UUID for legacy or compound identifiers. It is for
// compatibility keys, not security-sensitive identifiers.
func Derive(namespace, value string) string {
	sum := md5.Sum([]byte(namespace + ":" + value))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return formatUUID(sum)
}

// Public converts a canonical UUID to a compact, case-insensitive 26-character
// Crockford Base32 identifier suitable for URLs.
func Public(id string) (string, error) {
	raw, err := parseUUID(id)
	if err != nil {
		return "", err
	}
	var out [26]byte
	var buffer uint32
	var bits uint
	output := 0

	// A UUID has 128 bits while the encoded form has 130. Prefix two zero bits.
	bits = 2
	for _, b := range raw {
		buffer = (buffer << 8) | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[output] = crockfordAlphabet[(buffer>>bits)&31]
			output++
			if bits == 0 {
				buffer = 0
			} else {
				buffer &= (1 << bits) - 1
			}
		}
	}
	return string(out[:]), nil
}

// Parse accepts either a canonical UUID or a compact public identifier and
// returns the canonical UUID form.
func Parse(value string) (string, error) {
	value = strings.TrimSpace(value)
	if raw, err := parseUUID(value); err == nil {
		return formatUUID(raw), nil
	}
	if len(value) != 26 {
		return "", errors.New("invalid entity id")
	}

	var raw [16]byte
	var buffer uint32
	var bits uint
	output := 0
	for index, ch := range strings.ToUpper(value) {
		n, ok := crockfordValue(byte(ch))
		if !ok || (index == 0 && n > 7) {
			return "", errors.New("invalid public entity id")
		}
		buffer = (buffer << 5) | uint32(n)
		bits += 5
		if index == 0 {
			// Discard the two leading padding bits.
			bits = 3
			buffer &= 7
		}
		for bits >= 8 {
			bits -= 8
			if output >= len(raw) {
				return "", errors.New("invalid public entity id")
			}
			raw[output] = byte(buffer >> bits)
			output++
			if bits == 0 {
				buffer = 0
			} else {
				buffer &= (1 << bits) - 1
			}
		}
	}
	if output != len(raw) || bits != 0 {
		return "", errors.New("invalid public entity id")
	}
	return formatUUID(raw), nil
}

func incrementRandom() {
	for index := len(lastRandom) - 1; index >= 0; index-- {
		lastRandom[index]++
		if lastRandom[index] != 0 {
			return
		}
	}
	if _, err := rand.Read(lastRandom[:]); err != nil {
		panic("entityid: secure random source unavailable: " + err.Error())
	}
}

func parseUUID(value string) ([16]byte, error) {
	var raw [16]byte
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(compact) != 32 {
		return raw, errors.New("invalid UUID")
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return raw, errors.New("invalid UUID")
	}
	copy(raw[:], decoded)
	return raw, nil
}

func formatUUID(raw [16]byte) string {
	var out [36]byte
	hex.Encode(out[0:8], raw[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], raw[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], raw[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], raw[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], raw[10:16])
	return string(out[:])
}

func crockfordValue(ch byte) (byte, bool) {
	switch ch {
	case 'O':
		ch = '0'
	case 'I', 'L':
		ch = '1'
	}
	index := strings.IndexByte(crockfordAlphabet, ch)
	return byte(index), index >= 0
}
