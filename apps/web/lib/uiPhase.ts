import type { Snapshot, UIPhase } from '../features/game/model/types';

type Params = {
  snapshot: Snapshot | null;
  status: string;
};

export function deriveUIPhase({ snapshot, status }: Params): UIPhase {
  if (!snapshot) {
    return status === 'queueing' ||
      status === 'queued' ||
      status === 'matched' ||
      status === 'matched_connecting' ||
      status === 'recovering'
      ? 'queueing'
      : 'lobby';
  }

  if (snapshot.state === 'ended') {
    return 'match_end';
  }

  if (snapshot.phase === 'round_result') {
    return 'round_result';
  }

  if (snapshot.roundPhase === 'round_transition') {
    return 'round_result';
  }

  if (snapshot.phase === 'live' && snapshot.roundPhase === 'round_intro') {
    return 'prematch_countdown';
  }

  if (snapshot.phase === 'live') {
    return 'live_round';
  }

  return 'lobby';
}
