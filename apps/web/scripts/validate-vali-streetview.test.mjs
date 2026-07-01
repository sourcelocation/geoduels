import assert from "node:assert/strict";
import test from "node:test";
import { buildMetadataUrl, inspectLocations, parseArgs, validateLocation } from "./validate-vali-streetview.mjs";

test("parseArgs reads numeric and boolean options", () => {
  const options = parseArgs([
    "--input", "in.json",
    "--output", "out.json",
    "--api-key", "test-key",
    "--concurrency", "3",
    "--retries", "0",
  ]);
  assert.equal(options.concurrency, 3);
  assert.equal(options.retries, 0);
  assert.equal(options.repairRadius, 50);
});

test("inspectLocations accepts Vali output and rejects unsafe rows", () => {
  const valid = { lat: 10, lng: 20, heading: 45, panoId: "pano-1", extra: { tags: ["XX"] } };
  const result = inspectLocations([
    valid,
    { ...valid },
    { lat: 91, lng: 20, panoId: "pano-2" },
    { lat: 10, lng: 20 },
  ]);

  assert.deepEqual(result.candidates, [{ index: 0, location: valid }]);
  assert.deepEqual(result.rejected.map(({ reason }) => reason), [
    "duplicate_pano_id",
    "invalid_lat",
    "missing_pano_id",
  ]);
});

test("buildMetadataUrl encodes metadata parameters", () => {
  const url = buildMetadataUrl("key value", { location: "1.2,3.4", radius: 50, source: "outdoor" });
  assert.equal(url.origin + url.pathname, "https://maps.googleapis.com/maps/api/streetview/metadata");
  assert.equal(url.searchParams.get("key"), "key value");
  assert.equal(url.searchParams.get("location"), "1.2,3.4");
});

test("validateLocation keeps a working panorama", async () => {
  const fetchImpl = async () => new Response(JSON.stringify({
    status: "OK",
    pano_id: "pano-1",
    location: { lat: 10, lng: 20 },
  }));
  const result = await validateLocation(
    { index: 2, location: { lat: 10, lng: 20, panoId: "pano-1" } },
    { apiKey: "key", repairRadius: 50, retries: 0, timeoutMs: 1000 },
    fetchImpl,
  );
  assert.equal(result.ok, true);
  assert.equal(result.status, "OK");
});

test("validateLocation refreshes a deleted panorama from coordinates", async () => {
  const requests = [];
  const fetchImpl = async (url) => {
    requests.push(url);
    return new Response(JSON.stringify(requests.length === 1
      ? { status: "ZERO_RESULTS" }
      : { status: "OK", pano_id: "pano-2", location: { lat: 10.1, lng: 20.1 } }));
  };
  const result = await validateLocation(
    { index: 3, location: { lat: 10, lng: 20, panoId: "pano-old" } },
    { apiKey: "key", repairRadius: 50, retries: 0, timeoutMs: 1000 },
    fetchImpl,
  );
  assert.equal(result.status, "PANO_ID_REFRESHED");
  assert.equal(result.returnedPanoId, "pano-2");
  assert.equal(requests[1].searchParams.get("source"), "outdoor");
});

test("validateLocation does not retry a denied API key", async () => {
  let calls = 0;
  const fetchImpl = async () => {
    calls += 1;
    return new Response(JSON.stringify({ status: "REQUEST_DENIED", error_message: "bad key" }));
  };
  await assert.rejects(
    validateLocation(
      { index: 4, location: { lat: 10, lng: 20, panoId: "pano-1" } },
      { apiKey: "key", repairRadius: 50, retries: 5, timeoutMs: 1000 },
      fetchImpl,
    ),
    /REQUEST_DENIED: bad key/,
  );
  assert.equal(calls, 1);
});
