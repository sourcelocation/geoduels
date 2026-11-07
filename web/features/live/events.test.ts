import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { eventChecks } from "./events";

// Written by the server's tests from the events it really sends.
const examples: Array<{ type: string; data: unknown }> = JSON.parse(
  readFileSync(resolve(__dirname, "../../../tests/shared/live-events.json"), "utf8"),
);

function checkFor(type: string) {
  const entry = Object.entries(eventChecks).find(([name]) => name === type);
  if (!entry) throw new Error(`no check for ${type}`);
  return entry[1];
}

describe("live events", () => {
  it("has an example of every event the client listens for", () => {
    expect(examples.map((example) => example.type).sort()).toEqual(Object.keys(eventChecks).sort());
  });

  it.each(examples)("understands $type", ({ type, data }) => {
    expect(checkFor(type)(data)).toBe(true);
  });

  it.each(examples)("refuses a malformed $type", ({ type }) => {
    expect(checkFor(type)("nonsense")).toBe(false);
  });
});
