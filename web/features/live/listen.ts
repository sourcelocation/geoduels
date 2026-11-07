import { authGateway } from "../auth/auth-gateway";
import type { HomeRuntime } from "../home/state/home-runtime";
import { inboxStore } from "../inbox/inbox-store";
import { socialStore } from "../social/social-store";
import { liveConnection } from "./live-connection";

/** Who hears what. Each event goes where its state lives. */
export function listenToLive(runtime: HomeRuntime) {
  const { partyController, matchController, matchRouteController, chatController, directChat, sessionController } = runtime;

  liveConnection.on("session", (session) => {
    // The cookie names someone else than this page thinks, like after signing in on another tab.
    if ((session.userId ?? "") !== sessionController.getState().userId) {
      void authGateway.bootstrap({ force: true }).catch(() => undefined);
    }
    socialStore.replace(session.social);
    inboxStore.replace(session.inbox);
    partyController.applySession(session.party);
    matchController.applySession(session.queue, session.activeMatch);
    authGateway.applyGlobal(session.status);
    chatController.reload();
    directChat.reload();
  });
  liveConnection.on("status", (status) => authGateway.applyGlobal(status));

  liveConnection.on("social.updated", (social) => socialStore.replace(social));
  liveConnection.on("friend.presence", (presence) => socialStore.applyPresence(presence));

  liveConnection.on("inbox.added", ({ message }) => {
    inboxStore.add(message);
    // Whatever it says, the account may have changed with it.
    void authGateway.bootstrap({ force: true }).catch(() => undefined);
  });
  liveConnection.on("inbox.read", ({ id }) => inboxStore.applyRead(id));

  liveConnection.on("conversation.item", ({ item, notify }) => {
    chatController.receive(item);
    directChat.receive(item);
    if (notify && !item.actions?.length && directChat.getState().conversationId !== item.conversationId) {
      socialStore.addUnread(item.conversationId);
    }
  });
  liveConnection.on("conversation.item_removed", ({ conversationId, itemId }) => {
    chatController.remove(conversationId, itemId);
    directChat.remove(conversationId, itemId);
  });

  liveConnection.on("party.updated", ({ party }) => partyController.applySnapshot(party));
  liveConnection.on("party.left", ({ partyId }) => partyController.applyLeft(partyId));

  liveConnection.on("queue.updated", (queue) => matchController.applyQueue(queue));
  liveConnection.on("match.started", ({ match }) => matchRouteController.acceptStartedMatch(match.matchId));
}
