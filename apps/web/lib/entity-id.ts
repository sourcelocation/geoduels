const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";
const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function toPublicEntityId(value: string): string {
  const normalized = value.trim();
  if (!UUID_PATTERN.test(normalized)) return normalized;
  let number = BigInt(`0x${normalized.replaceAll("-", "")}`);
  let encoded = "";
  for (let index = 0; index < 26; index += 1) {
    encoded = CROCKFORD[Number(number & 31n)] + encoded;
    number >>= 5n;
  }
  return encoded;
}

export function fromPublicEntityId(value: string): string | null {
  const normalized = value.trim();
  if (UUID_PATTERN.test(normalized)) return normalized.toLowerCase();
  if (normalized.length !== 26) return null;

  let number = 0n;
  for (const rawCharacter of normalized.toUpperCase()) {
    const character =
      rawCharacter === "O"
        ? "0"
        : rawCharacter === "I" || rawCharacter === "L"
          ? "1"
          : rawCharacter;
    const digit = CROCKFORD.indexOf(character);
    if (digit < 0) return null;
    number = (number << 5n) | BigInt(digit);
  }
  if (number >= 1n << 128n) return null;
  const hex = number.toString(16).padStart(32, "0");
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join("-");
}

export function normalizeEntityRouteId(value: string): string {
  return fromPublicEntityId(value) || value.trim();
}
