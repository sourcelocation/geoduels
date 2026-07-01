import dynamic from 'next/dynamic';
import type { HomeModel } from '../model/types';
import type { LobbyContentRoute } from '../../../components/ui/LobbyScreen';
import HomePageChatDock from './HomePageChatDock';
import HomePageLobby from './HomePageLobby';
import HomePageOverlays from './HomePageOverlays';
import { loadHomePageGame } from './lobby-preloading';

const HomePageGame = dynamic(loadHomePageGame, {
  ssr: false,
});

type HomePageViewProps = {
  model: HomeModel;
  lobbyRoute?: LobbyContentRoute;
  mapId?: string;
};

export default function HomePageView({ model, lobbyRoute = 'play', mapId = '' }: HomePageViewProps) {
  const showGame =
    model.view.game.inGame &&
    !(
      model.view.game.uiPhase === 'match_end' &&
      model.view.game.showMatchEndPage
    );

  return (
    <main className="relative min-h-screen overflow-hidden text-ink">
      <HomePageOverlays auth={model.view.auth} overlays={model.view.overlays} actions={model.actions} />
      <HomePageLobby auth={model.view.auth} lobby={model.view.lobby} meta={model.view.meta} actions={model.actions} contentRoute={lobbyRoute} mapId={mapId} />
      <HomePageChatDock chat={model.view.chat} actions={model.actions} />
      {showGame ? (
        <HomePageGame
          game={model.view.game}
          maxHP={model.view.meta.maxHP}
          actions={model.actions}
        />
      ) : null}
    </main>
  );
}
