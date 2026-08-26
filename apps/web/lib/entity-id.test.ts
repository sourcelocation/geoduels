import { describe, expect, it } from "vitest";
import {
  fromPublicEntityId,
  normalizeEntityRouteId,
  toPublicEntityId,
} from "./entity-id";

describe("entity route ids", () => {
  it("round-trips UUIDs through compact Crockford Base32", () => {
    const uuid = "01976ad2-9c42-7d31-a820-63d7c5279841";
    const publicId = toPublicEntityId(uuid);
    expect(publicId).toBe("01JXND5722FMRTG833TZ2JF621");
    expect(fromPublicEntityId(publicId.toLowerCase())).toBe(uuid);
  });

  it("preserves semantic slugs for display but rejects legacy route ids", () => {
    expect(toPublicEntityId("a-source-world")).toBe("a-source-world");
    expect(normalizeEntityRouteId("match-legacy")).toBe("");
  });

  it("accepts Crockford ambiguous characters", () => {
    const uuid = "00000000-0000-7000-8000-000000000001";
    const publicId = toPublicEntityId(uuid)
      .replaceAll("0", "O")
      .replaceAll("1", "L");
    expect(fromPublicEntityId(publicId)).toBe(uuid);
  });
});
