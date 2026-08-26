import { useEffect, useState } from 'react';
import { Button } from '../../../components/ui/button';

type Props = {
  connected: boolean;
  accessToken: string;
  status: string;
  joinQueue: () => void;
  cancelQueue: () => void;
  queueError: string;
};

function formatQueueElapsed(ms: number) {
  const totalSeconds = ms > 0 ? Math.max(1, Math.ceil(ms / 1000)) : 0;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

export default function QueueCard({ connected, accessToken, status, joinQueue, cancelQueue, queueError }: Props) {
  const [queueStartedAt, setQueueStartedAt] = useState<number | null>(null);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const disabled = !accessToken || status === 'queueing';
  const isQueueing = status === 'queueing';
  const showConnectionError = !connected && queueError.toLowerCase() === 'connection error';
  const primaryLabel = showConnectionError ? 'Connection Error' : isQueueing ? 'Finding Opponent...' : 'PLAY';
  const queueElapsedLabel = formatQueueElapsed(queueStartedAt ? nowMs - queueStartedAt : 0);

  useEffect(() => {
    if (!isQueueing) {
      setQueueStartedAt(null);
      return;
    }
    setQueueStartedAt((current) => current ?? Date.now());
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [isQueueing]);

  return (
    <section className="relative w-full max-w-[540px] overflow-hidden rounded-2xl border border-status-success/30 bg-surface-panel p-6 text-content-primary shadow-elev-4 md:p-7">
      <div className="pointer-events-none absolute inset-0 bg-status-success/10" />
      <div className="pointer-events-none absolute inset-0 bg-brand-blue/10" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-1/2 bg-surface-overlay bg-[url('/mountains.v1.svg')] bg-cover bg-center opacity-90" />

      <div className="relative z-content">
        <span className="text-label font-strong uppercase tracking-display-max text-content-secondary">Ranked</span>
        <h3 className="mt-2 font-display text-display-lg font-strong leading-collapsed tracking-heading text-content-primary">Duel</h3>
        <p className="mt-2 text-heading-md leading-collapsed text-status-success/90">Moving allowed</p>
        <Button
          variant="primary"
          type="button"
          onClick={joinQueue}
          disabled={disabled}
          className="mt-8 flex min-h-14 w-full items-center justify-center gap-3 rounded-full border border-status-success/35 bg-action-primary px-6 py-3 text-heading-md font-strong uppercase tracking-eyebrow text-content-on-action shadow-elev-2 transition hover:bg-action-primary-hover disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="text-heading-sm leading-collapsed">▶</span>
          {primaryLabel}
        </Button>
        {isQueueing && (
          <Button
            variant="secondary"
            type="button"
            onClick={cancelQueue}
            className="mt-3 min-h-11 w-full rounded-full border border-border-default bg-surface-inset px-5 py-3 text-label font-strong uppercase tracking-control text-content-primary transition hover:bg-surface-fill"
          >
            <span className="text-status-success">{queueElapsedLabel}</span>
          </Button>
        )}
        {queueError && <p className="mt-3 text-body-sm font-semibold text-status-danger">{queueError}</p>}
      </div>
    </section>
  );
}
