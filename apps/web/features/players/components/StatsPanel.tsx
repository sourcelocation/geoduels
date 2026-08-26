type Props = {
  mmr: number;
  gamesPlayed: number;
  winsPct: number;
};

export default function StatsPanel({ mmr, gamesPlayed, winsPct }: Props) {
  return (
    <section className="rounded-xl border border-border-default bg-surface-panel p-5 text-content-primary shadow-elev-2">
      <h2 className="text-body-sm font-strong uppercase tracking-eyebrow-wide text-content-secondary">Profile Stats</h2>
      <div className="mt-4 grid grid-cols-3 gap-3">
        <div className="rounded-md border border-border-default bg-surface-raised p-3">
          <p className="text-label uppercase tracking-eyebrow text-content-secondary">Current Rating</p>
          <p className="mt-1 text-heading-md font-strong text-content-primary">{mmr}</p>
        </div>
        <div className="rounded-md border border-border-default bg-surface-raised p-3">
          <p className="text-label uppercase tracking-eyebrow text-content-secondary">Total Games</p>
          <p className="mt-1 text-heading-md font-strong text-content-primary">{gamesPlayed}</p>
        </div>
        <div className="rounded-md border border-border-default bg-surface-raised p-3">
          <p className="text-label uppercase tracking-eyebrow text-content-secondary">Win Rate</p>
          <p className="mt-1 text-heading-md font-strong text-content-primary">{winsPct}%</p>
        </div>
      </div>
    </section>
  );
}
