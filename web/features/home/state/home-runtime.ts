
import { createSfxController } from '../../../lib/audio/browser-sfx-controller';
import type { SfxController } from '../../../lib/audio/sfx';
import { SessionController } from '../../auth/controllers/session-controller';
import { authGateway } from '../../auth/auth-gateway';
import { ChatController } from '../../chat/controllers/chat-controller';
import { listenToLive } from '../../live/listen';
import { socialStore } from '../../social/social-store';
import { GameController } from '../../game/controllers/game-controller';
import { PartyController } from '../../lobby/controllers/party-controller';
import { MatchController } from '../../matchmaking/controllers/match-controller';
import { MatchRouteController } from '../../matchmaking/controllers/match-route-controller';

export type HomeRuntime = {
  sessionController: SessionController;
  partyController: PartyController;
  matchController: MatchController;
  matchRouteController: MatchRouteController;
  gameController: GameController;
  // The party or match the player is in.
  chatController: ChatController;
  // The conversation open with one other person.
  directChat: ChatController;
  sfxController: SfxController;
  started: boolean;
};

function createHomeRuntime(): HomeRuntime {
  const runtime = {} as HomeRuntime;
  runtime.started = false;
  runtime.sfxController = createSfxController();
  runtime.sessionController = new SessionController({
    authGateway: authGateway,
    onResetSession: () => {
      runtime.matchController.resetConnectionState();
      runtime.partyController?.reset();
      runtime.matchRouteController?.reset();
      runtime.chatController?.close();
      runtime.directChat?.close();
    }
  });
  runtime.matchController = new MatchController({
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController
  });
  runtime.partyController = new PartyController({
    sessionController: runtime.sessionController
  });
  runtime.matchRouteController = new MatchRouteController({
    sessionController: runtime.sessionController,
    matchController: runtime.matchController
  });
  runtime.gameController = new GameController({
    matchController: runtime.matchController,
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController
  });
  runtime.chatController = new ChatController({
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController
  });
  runtime.directChat = new ChatController({
    sessionController: runtime.sessionController,
    sfxController: runtime.sfxController,
    onRead: (conversationId) => socialStore.markRead(conversationId)
  });
  return runtime;
}

export const homeRuntime = createHomeRuntime();

export function startHomeRuntime(runtime: HomeRuntime) {
  if (runtime.started) return;
  runtime.started = true;
  runtime.sessionController.start();
  runtime.matchController.start();
  runtime.matchRouteController.start();
  runtime.sfxController.start();
  runtime.gameController.start();
  listenToLive(runtime);
}
