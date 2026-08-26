import ChatPanel from "../../chat/components/ChatPanel";
import type { HomeActions, HomeChatView } from "../model/types";

type HomePageChatDockProps = {
  chat: HomeChatView;
  actions: Pick<HomeActions, "sendChatMessage" | "sendChatEmote">;
};

export default function HomePageChatDock({ chat, actions }: HomePageChatDockProps) {
  if (!chat.conversationId || !chat.selfUserId) return null;

  return (
    <ChatPanel
      messages={chat.messages}
      selfUserId={chat.selfUserId}
      teamId={chat.teamId}
      onSendMessage={actions.sendChatMessage}
      onSendEmote={actions.sendChatEmote}
      className="app-layer-chat fixed left-3 top-24 w-[min(calc(100vw-1.5rem),21rem)] md:left-4 md:top-28"
    />
  );
}
