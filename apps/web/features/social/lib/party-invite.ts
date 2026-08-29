export const PARTY_INVITE_RESEND_MS = 30_000;

export function partyInviteCanResend(createdAt?: string, now = Date.now()) {
  if (!createdAt) return true;
  const sentAt = Date.parse(createdAt);
  if (Number.isNaN(sentAt)) return false;
  return now - sentAt >= PARTY_INVITE_RESEND_MS;
}

export function partyInviteResendInMs(createdAt?: string, now = Date.now()) {
  if (!createdAt) return 0;
  const sentAt = Date.parse(createdAt);
  if (Number.isNaN(sentAt)) return PARTY_INVITE_RESEND_MS;
  return Math.max(0, sentAt + PARTY_INVITE_RESEND_MS - now);
}
