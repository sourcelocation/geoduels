import { describe, expect, it } from "vitest";
import { partyInviteCanResend, partyInviteResendInMs } from "./party-invite";

describe("party invite resend window", () => {
  it("allows a first invite and blocks a resend for 30 seconds", () => {
    expect(partyInviteCanResend()).toBe(true);
    const createdAt = new Date(1_000).toISOString();
    expect(partyInviteCanResend(createdAt, 1_000)).toBe(false);
    expect(partyInviteResendInMs(createdAt, 1_000)).toBe(30_000);
    expect(partyInviteCanResend(createdAt, 31_000)).toBe(true);
    expect(partyInviteResendInMs(createdAt, 31_000)).toBe(0);
  });
});
