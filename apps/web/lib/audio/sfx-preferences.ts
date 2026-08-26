type SfxMuteListener = (muted: boolean) => void;

let sfxMuted = false;
const listeners = new Set<SfxMuteListener>();

export function isSfxMuted() {
  return sfxMuted;
}

export function setSfxMuted(muted: boolean) {
  if (sfxMuted === muted) return;
  sfxMuted = muted;
  listeners.forEach((listener) => listener(muted));
}

export function subscribeToSfxMute(listener: SfxMuteListener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
