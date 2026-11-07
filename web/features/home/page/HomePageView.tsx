import dynamic from 'next/dynamic';
import { useAppShellOptions } from '../../app-shell/components/AppShell';
import type { HomeModel } from '../model/types';
import type { LobbyContentRoute } from '../../lobby/components/LobbyScreen';
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
  routeLoading?: boolean;
};

export default function HomePageView({ model, lobbyRoute = 'play', mapId = '', routeLoading = false }: HomePageViewProps) {
  const showGame =
    model.view.game.inGame &&
    !(
      model.view.game.uiPhase === 'match_end' &&
      model.view.game.showMatchEndPage
    );

  useAppShellOptions({ hidden: model.view.lobby.inGame });

  return (
    <main className="relative flex min-h-0 flex-1 flex-col text-content-primary">
      <HomePageOverlays auth={model.view.auth} overlays={model.view.overlays} maxHP={model.view.meta.maxHP} actions={model.actions} />
      <HomePageLobby auth={model.view.auth} lobby={model.view.lobby} meta={model.view.meta} actions={model.actions} contentRoute={lobbyRoute} mapId={mapId} routeLoading={routeLoading} />
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
