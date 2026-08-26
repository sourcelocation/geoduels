import AppModalShell from "../../../../components/ui/AppModalShell";
import { CenteredSpinner } from "../../../../components/ui/Spinner";
import type { MatchConfig } from "../../../matchmaking/lib/queue-client";
import type { CustomMap, MapScope, MapSort } from "../../../maps/lib/maps-client";
import { LobbyPlaceholder, LobbySectionHeader } from "../lobby-primitives";
import {
  MapCard,
  MapScopeNav,
  MapSearchControl,
  MapSortControl,
  type MapScopeLabel,
} from "./MapPanels";

export function MapPickerModal({
  canUploadCustomMaps,
  hasMapSearch,
  partyConfig,
  mapScope,
  mapScopeLabels,
  mapSearchInput,
  mapSort,
  mapsLoading,
  onClose,
  readyMaps,
  selectMapForParty,
  setMapScope,
  setMapSearchInput,
  setMapSort,
  thumbnailURL,
}: {
  canUploadCustomMaps: boolean;
  hasMapSearch: boolean;
  partyConfig: MatchConfig;
  mapScope: MapScope;
  mapScopeLabels: MapScopeLabel[];
  mapSearchInput: string;
  mapSort: MapSort;
  mapsLoading: boolean;
  onClose: () => void;
  readyMaps: CustomMap[];
  selectMapForParty: (item: CustomMap) => void;
  setMapScope: (scope: MapScope) => void;
  setMapSearchInput: (value: string) => void;
  setMapSort: (sort: MapSort) => void;
  thumbnailURL: (item: Pick<CustomMap, "thumbnailVariant" | "thumbnailKey">) => string;
}) {
  return (
    <AppModalShell title="Select Map" onClose={onClose} placement="center" maxWidthClassName="max-w-[1040px]" contentClassName="space-y-4">
      <div className="grid gap-4 lg:grid-cols-[190px_minmax(0,1fr)]">
        <aside className="grid gap-2 rounded-xl border border-border-default bg-surface-grouped p-3 lg:content-start">
          <MapScopeNav labels={mapScopeLabels} value={mapScope} onChange={setMapScope} />
        </aside>
        <section className="min-w-0">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <LobbySectionHeader eyebrow="Party Map" title={mapScopeLabels.find((item) => item.scope === mapScope)?.label} />
            <div className="flex w-full flex-col gap-3 sm:w-auto sm:items-end">
              <MapSearchControl id="party-map-search" value={mapSearchInput} onChange={setMapSearchInput} />
              {mapScope === "community" ? <MapSortControl value={mapSort} onChange={setMapSort} /> : null}
            </div>
          </div>
          {mapScope === "mine" && !canUploadCustomMaps ? (
            <LobbyPlaceholder className="mt-5">Sign in with a permanent account to use your custom maps.</LobbyPlaceholder>
          ) : mapsLoading ? (
            <CenteredSpinner label="Loading maps" className="mt-6" />
          ) : readyMaps.length === 0 ? (
            <LobbyPlaceholder className="mt-6 p-8">{hasMapSearch ? "No ready maps match your search." : "No ready maps in this section yet."}</LobbyPlaceholder>
          ) : (
            <div className="mt-5 grid max-h-[56vh] gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-3">
              {readyMaps.map((item) => (
                <div key={item.id} className="overflow-hidden rounded-2xl">
                  <MapCard item={item} mode="select" selected={item.id === partyConfig.mapId} thumbnailURL={thumbnailURL} onSelect={selectMapForParty} />
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </AppModalShell>
  );
}
