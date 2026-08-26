import { sfxRegistry, type SfxController, type SfxDefinition, type SfxName, type SfxSource } from './sfx';
import { isSfxMuted, subscribeToSfxMute } from './sfx-preferences';

type ManagedSource = {
  source: AudioBufferSourceNode;
  gain: GainNode;
};

function pickPlayableSource(audio: HTMLAudioElement, sources: readonly SfxSource[]) {
  for (const source of sources) {
    if (!source.type || audio.canPlayType(source.type)) {
      return source.src;
    }
  }
  return sources[0]?.src || '';
}

function getAudioContextCtor() {
  if (typeof window === 'undefined') return null;
  return window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext || null;
}

export class BrowserSfxController implements SfxController {
  private readonly probe = typeof Audio !== 'undefined' ? new Audio() : null;
  private audioContext: AudioContext | null = null;
  private masterGain: GainNode | null = null;
  private readonly buffers = new Map<SfxName, AudioBuffer>();
  private readonly loading = new Map<SfxName, Promise<AudioBuffer | null>>();
  private readonly managed = new Map<SfxName, ManagedSource>();
  private unlocked = false;
  private started = false;
  private destroyed = false;
  private unsubscribeFromMute: (() => void) | null = null;

  private readonly unlock = () => {
    void this.unlockAudio();
  };

  start() {
    if (this.started || typeof window === 'undefined' || !this.probe) return;
    this.destroyed = false;
    this.started = true;
    this.unsubscribeFromMute = subscribeToSfxMute((muted) => {
      if (muted) this.stopAllManaged();
    });
    window.addEventListener('pointerdown', this.unlock, { once: true });
    window.addEventListener('keydown', this.unlock, { once: true });
  }

  destroy() {
    if (typeof window === 'undefined') return;
    this.destroyed = true;
    this.started = false;
    window.removeEventListener('pointerdown', this.unlock);
    window.removeEventListener('keydown', this.unlock);
    this.stopAllManaged();
    this.buffers.clear();
    this.loading.clear();
    this.masterGain?.disconnect();
    this.masterGain = null;
    if (this.audioContext && this.audioContext.state !== 'closed') {
      void this.audioContext.close().catch(() => {});
    }
    this.audioContext = null;
    this.unlocked = false;
    this.unsubscribeFromMute?.();
    this.unsubscribeFromMute = null;
  }

  play(name: SfxName) {
    void this.playBuffer(name, false, false);
  }

  playManaged(name: SfxName) {
    void this.playBuffer(name, true, false);
  }

  playLoop(name: SfxName) {
    void this.playBuffer(name, true, true);
  }

  stop(name: SfxName) {
    const managed = this.managed.get(name);
    if (!managed) return;
    this.managed.delete(name);
    managed.source.onended = null;
    try {
      managed.source.stop();
    } catch {}
    this.disconnectManaged(managed);
  }

  private async unlockAudio() {
    if (this.destroyed || typeof window === 'undefined' || !this.probe) return;
    window.removeEventListener('pointerdown', this.unlock);
    window.removeEventListener('keydown', this.unlock);
    const context = this.ensureAudioContext();
    if (!context) return;
    this.unlocked = true;
    if (context.state === 'suspended') {
      await context.resume().catch(() => {});
    }
    this.preloadAll();
  }

  private ensureAudioContext() {
    if (this.audioContext) return this.audioContext;
    const AudioContextCtor = getAudioContextCtor();
    if (!AudioContextCtor) return null;
    const context = new AudioContextCtor();
    const masterGain = context.createGain();
    masterGain.gain.value = 1;
    masterGain.connect(context.destination);
    this.audioContext = context;
    this.masterGain = masterGain;
    return context;
  }

  private async playBuffer(name: SfxName, managed: boolean, loop: boolean) {
    if (this.destroyed || !this.probe || isSfxMuted()) return;
    const context = this.ensureAudioContext();
    if (!context || !this.masterGain) return;
    if (!this.unlocked) {
      await this.unlockAudio();
    } else if (context.state === 'suspended') {
      await context.resume().catch(() => {});
    }
    const buffer = this.buffers.get(name) || (await this.loadBuffer(name));
    if (!buffer || this.destroyed || isSfxMuted() || !this.audioContext || !this.masterGain) return;
    if (managed) {
      this.stop(name);
    }

    const definition = sfxRegistry[name];
    const source = this.audioContext.createBufferSource();
    const gain = this.audioContext.createGain();
    source.buffer = buffer;
    source.loop = loop;
    gain.gain.value = definition.volume ?? 1;
    source.connect(gain);
    gain.connect(this.masterGain);

    if (managed) {
      this.managed.set(name, { source, gain });
      source.onended = () => {
        if (this.managed.get(name)?.source === source) {
          this.managed.delete(name);
        }
        this.disconnectManaged({ source, gain });
      };
    } else {
      source.onended = () => {
        this.disconnectManaged({ source, gain });
      };
    }

    try {
      source.start();
    } catch {
      source.disconnect();
      gain.disconnect();
      if (managed && this.managed.get(name)?.source === source) {
        this.managed.delete(name);
      }
    }
  }

  private preloadAll() {
    if (!this.unlocked) return;
    (Object.keys(sfxRegistry) as SfxName[]).forEach((name) => {
      void this.loadBuffer(name);
    });
  }

  private loadBuffer(name: SfxName) {
    const cached = this.buffers.get(name);
    if (cached) return Promise.resolve(cached);
    const inFlight = this.loading.get(name);
    if (inFlight) return inFlight;

    const promise = this.fetchAndDecode(name)
      .catch(() => null)
      .finally(() => {
        this.loading.delete(name);
      });
    this.loading.set(name, promise);
    return promise;
  }

  private async fetchAndDecode(name: SfxName) {
    const context = this.ensureAudioContext();
    if (!context || !this.probe) return null;
    const src = this.resolveSource(sfxRegistry[name]);
    if (!src) return null;
    const response = await fetch(src);
    if (!response.ok) return null;
    const data = await response.arrayBuffer();
    const buffer = await context.decodeAudioData(data.slice(0));
    this.buffers.set(name, buffer);
    return buffer;
  }

  private resolveSource(definition: SfxDefinition) {
    if (!this.probe) return '';
    return pickPlayableSource(this.probe, definition.sources);
  }

  private stopAllManaged() {
    Array.from(this.managed.keys()).forEach((name) => this.stop(name));
  }

  private disconnectManaged(managed: ManagedSource) {
    try {
      managed.source.disconnect();
    } catch {}
    try {
      managed.gain.disconnect();
    } catch {}
  }
}

export function createSfxController(): SfxController {
  return new BrowserSfxController();
}
