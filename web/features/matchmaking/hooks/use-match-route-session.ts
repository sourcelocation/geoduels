import { useEffect, useSyncExternalStore } from 'react';
import { homeRuntime } from '../../home/state/home-runtime';

export function useMatchRouteSession(matchId: string | null) {
  const runtime = homeRuntime;
  const controller = runtime.matchRouteController;
  const state = useSyncExternalStore(
    controller.subscribe,
    controller.getState.bind(controller),
    controller.getState.bind(controller)
  );

  useEffect(() => {
    controller.setTargetMatch(matchId);
    return () => {
      controller.reset();
    };
  }, [controller, matchId]);

  return state;
}
