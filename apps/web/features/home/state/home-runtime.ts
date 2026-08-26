import type { RuntimeConfig } from '../../../lib/runtime-config';
import { getRuntimeConfig } from '../../../lib/runtime-config';
import { createSfxController } from '../../../lib/audio/browser-sfx-controller';
import type { SfxController } from '../../../lib/audio/sfx';
import { SessionController } from '../../auth/controllers/session-controller';
import { getAuthGateway } from '../../auth/auth-gateway';
import { ChatController } from '../../chat/controllers/chat-controller';
import { GameController } from '../../game/controllers/game-controller';
import { PartyController } from '../../lobby/controllers/party-controller';
import { MatchController } from '../../matchmaking/controllers/match-controller';
import { MatchRouteController } from '../../matchmaking/controllers/match-route-controller';

export type HomeRuntime = {
  config: RuntimeConfig;
  sessionController: SessionController;
  partyController: PartyController;
  matchController: MatchController;
  matchRouteController: MatchRouteController;
  gameController: GameController;
  chatController: ChatController;
  sfxController: SfxController;
  started: boolean;
};

let browserRuntime: HomeRuntime | null = null;

function createHomeRuntime(config: RuntimeConfig): HomeRuntime {
  const runtime = {} as HomeRuntime;
  runtime.config = config;
  runtime.started = false;
  runtime.sfxController = createSfxController();
  runtime.sessionController = new SessionController({
    config,
    authGateway: getAuthGateway(config),
    onResetSession: () => {
      runtime.matchController.resetConnectionState();
      runtime.partyController?.reset();
      runtime.chatController?.reset();
    }
  });
  runtime.matchController = new MatchController({
    config,
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController
  });
  runtime.partyController = new PartyController({
    config,
    sessionController: runtime.sessionController,
    matchController: runtime.matchController
  });
  runtime.matchRouteController = new MatchRouteController({
    config,
    sessionController: runtime.sessionController,
    matchController: runtime.matchController
  });
  runtime.gameController = new GameController({
    config,
    matchController: runtime.matchController,
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController
  });
  runtime.chatController = new ChatController({
    config,
    sfxController: runtime.sfxController
  });
  return runtime;
}

export function getHomeRuntime(config = getRuntimeConfig()) {
  if (typeof window === 'undefined') {
    return createHomeRuntime(config);
  }
  if (!browserRuntime) {
    browserRuntime = createHomeRuntime(config);
  }
  return browserRuntime;
}

export function startHomeRuntime(runtime: HomeRuntime) {
  if (runtime.started) return;
  runtime.started = true;
  runtime.sessionController.start();
  runtime.matchController.start();
  runtime.matchRouteController.start();
  runtime.sfxController.start();
  runtime.gameController.start();
}
