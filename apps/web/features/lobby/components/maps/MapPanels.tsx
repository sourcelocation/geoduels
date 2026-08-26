import Link from "next/link";
import React, { useState } from "react";
import { ArrowLeft, ChartNoAxesColumnIncreasing, Check, Heart, Map as MapIcon, Pencil, Play, Search, Star, Trophy, Upload, X } from "lucide-react";
import PlayerProfileLink from "../../../players/components/PlayerProfileLink";
import { Tooltip } from "../../../../components/ui/Tooltip";
import { Badge } from "../../../../components/ui/Badge";
import { Button, ButtonLink, IconButton } from "../../../../components/ui/button";
import { Tabs } from "../../../../components/ui/Tabs";
import { AppPanel, SectionCard } from "../../../../components/ui/compositions";
import { cn } from "../../../../lib/cn";
import { toPublicEntityId } from "../../../../lib/entity-id";
import type { CustomMap, GameplayMapRole, MapDetails, MapScope, MapSort, MapUpdateInput } from "../../../maps/lib/maps-client";
import {
  LobbyInput,
  LobbyPlaceholder,
  LobbyNotice,
  LobbySectionHeader,
} from "../lobby-primitives";
import { MapAdminOperations } from "./MapAdminOperations";
import { MapComments } from "./MapComments";
import { MapEditMetadataModal } from "./MapEditMetadataModal";
import { CenteredSpinner } from "../../../../components/ui/Spinner";
import { FileInputTrigger } from "../../../../components/ui/FileInputTrigger";

export type MapScopeLabel = { scope: MapScope; label: string };
type MapActionNotice = { title: string; message: string };

type MapSearchProps = {
  id: string;
  value: string;
  onChange: (value: string) => void;
};

export function MapSearchControl({ id, value, onChange }: MapSearchProps) {
  return (
    <div className="relative w-full sm:w-[260px]">
      <label htmlFor={id} className="sr-only">
        Search maps
      </label>
      <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-status-success" size={16} />
      <LobbyInput
        id={id}
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Search maps"
        className="h-11 w-full rounded-xl py-2 pl-9 pr-10 font-semibold"
      />
      {value ? (
        <IconButton
          aria-label="Clear map search"
          onClick={() => onChange("")}
          className="absolute right-2 top-1/2 h-7 min-h-7 w-7 -translate-y-1/2 border-transparent bg-transparent"
        >
          <X size={15} />
        </IconButton>
      ) : null}
    </div>
  );
}

export function MapScopeNav({
  labels,
  value,
  onChange,
}: {
  labels: MapScopeLabel[];
  value: MapScope;
  onChange: (scope: MapScope) => void;
}) {
  return (
    <div className="grid gap-2">
      {labels.map((item) => (
        <Button
          variant="ghost"
          key={item.scope}
          type="button"
          onClick={() => onChange(item.scope)}
          className={`rounded-lg px-4 py-3 text-left text-body-sm font-strong transition ${
            value === item.scope ? "bg-action-primary text-content-on-action" : "bg-surface-grouped text-content-secondary hover:bg-surface-fill hover:text-content-primary"
          }`}
        >
          {item.label}
        </Button>
      ))}
    </div>
  );
}

export function MapSortControl({
  value,
  onChange,
}: {
  value: MapSort;
  onChange: (sort: MapSort) => void;
}) {
  return (
    <Tabs
      appearance="segmented"
      value={value}
      onChange={onChange}
      items={[
        { id: "trending", label: "Trending" },
        { id: "popular", label: "Most Popular" },
        { id: "new", label: "New" },
      ]}
      aria-label="Sort maps"
    />
  );
}

function formatMapMetric(value: number) {
  if (value < 1000) return value.toLocaleString();
  return `${Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: value < 10000 ? 1 : 0 }).format(value)}+`;
}

const difficultyTone = { easy: "text-status-success", normal: "text-status-warning", hard: "text-status-danger" } satisfies Record<CustomMap["difficulty"], string>;

export function MapDifficulty({
  difficulty,
  className,
}: {
  difficulty: CustomMap["difficulty"];
  className?: string;
}) {
  return (
    <span
      className={cn("inline-flex items-center gap-1.5 text-label font-strong uppercase text-content-primary", className)}
      title="Difficulty"
    >
      <ChartNoAxesColumnIncreasing className={difficultyTone[difficulty]} size={15} />
      {difficulty}
    </span>
  );
}

export function MapCard({
  item,
  selected,
  mode,
  thumbnailURL,
  onSelect,
}: {
  item: CustomMap;
  selected?: boolean;
  mode: "link" | "select";
  thumbnailURL: (item: Pick<CustomMap, "thumbnailVariant" | "thumbnailKey">) => string;
  onSelect?: (item: CustomMap) => void;
}) {
  const content = (
    <AppPanel
      as="div"
      className={cn(
        "group relative aspect-[16/9] overflow-hidden rounded-xl text-left",
        "transition duration-normal hover:-translate-y-0.5",
        selected && "ring-2 ring-status-success",
      )}
    >
      <img
        src={thumbnailURL(item)}
        alt=""
        className="absolute inset-0 h-full w-full object-cover opacity-90 transition duration-emphasis group-hover:scale-105"
      />
      <div className="absolute inset-0 bg-gradient-to-b from-status-success/10 via-surface-raised/25 to-surface-page/90" />
      <div className="absolute inset-0 bg-gradient-to-r from-scrim via-transparent to-scrim" />
      <div className="absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-scrim via-scrim to-transparent" />

      <div className="absolute right-3 top-3 flex flex-wrap justify-end gap-2">
        {item.system || item.official ? (
          <Tooltip content="Official map">
            <span className="inline-flex h-6 w-6 items-center justify-center text-content-primary" aria-label="Official map">
              <Check size={14} strokeWidth={2.5} fill="none" aria-hidden="true" />
            </span>
          </Tooltip>
        ) : null}
        {item.modeMoving || item.modeNoMove || item.modeNmpz ? (
          <Tooltip content="Mode map">
            <span className="inline-flex h-6 w-6 items-center justify-center text-content-primary" aria-label="Mode map">
              <Trophy size={13} fill="none" strokeWidth={2.25} aria-hidden="true" />
            </span>
          </Tooltip>
        ) : null}
        {selected ? (
          <Badge tone="success" className="bg-action-primary text-content-on-action shadow-elev-1">Selected</Badge>
        ) : null}
      </div>

      <div className="absolute inset-x-0 bottom-0 grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3 p-4">
        <div className="min-w-0">
          <h3 className="truncate text-heading-sm font-strong leading-heading text-content-primary">{item.displayName}</h3>
          <p className="mt-1.5 truncate text-label font-strong text-content-secondary">{item.authorName || "GeoDuels"}</p>
        </div>
        <div className="grid gap-1.5 pb-0.5 text-label font-strong text-content-primary">
          <span className="inline-flex items-center justify-end gap-1.5" title="Locations">
            <MapIcon className="text-content-primary" size={15} />
            {formatMapMetric(item.locationCount)}
          </span>
          <span className="inline-flex items-center justify-end gap-1.5" title="Plays">
            <Play className="text-content-primary" size={15} />
            {formatMapMetric(item.playCount)}
          </span>
          <MapDifficulty difficulty={item.difficulty} className="justify-end" />
        </div>
      </div>
    </AppPanel>
  );

  if (mode === "select") {
    return (
      <Button type="button" variant="ghost" onClick={() => onSelect?.(item)} className="block w-full text-left">
        {content}
      </Button>
    );
  }
  return (
    <Link
      href={`/maps/${encodeURIComponent(toPublicEntityId(item.id))}`}
      className="block w-full text-left"
    >
      {content}
    </Link>
  );
}

export function MapsPanel({
  canUploadCustomMaps,
  hasMapSearch,
  mapScope,
  mapScopeLabels,
  mapSearchInput,
  mapSort,
  mapsLoading,
  mapActionNotice,
  partyActive,
  readyMaps,
  setMapScope,
  setMapSearchInput,
  setMapSort,
  selectMapForParty,
  thumbnailURL,
}: {
  canUploadCustomMaps: boolean;
  hasMapSearch: boolean;
  mapScope: MapScope;
  mapScopeLabels: MapScopeLabel[];
  mapSearchInput: string;
  mapSort: MapSort;
  mapsLoading: boolean;
  mapActionNotice: MapActionNotice | null;
  partyActive: boolean;
  readyMaps: CustomMap[];
  setMapScope: (scope: MapScope) => void;
  setMapSearchInput: (value: string) => void;
  setMapSort: (sort: MapSort) => void;
  selectMapForParty: (item: CustomMap) => void;
  thumbnailURL: (item: Pick<CustomMap, "thumbnailVariant" | "thumbnailKey">) => string;
}) {
  return (
    <AppPanel className="overflow-hidden rounded-2xl">
      <div className="grid lg:min-h-[min(640px,calc(100dvh-11rem))] lg:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="border-b border-border-default p-4 lg:border-b-0 lg:border-r">
          <div className="mb-4 flex items-center gap-2 text-content-primary">
            <MapIcon className="text-status-success" size={22} />
            <span className="text-heading-sm font-strong">Maps</span>
          </div>
          <MapScopeNav labels={mapScopeLabels} value={mapScope} onChange={setMapScope} />
        </aside>
        <section className="bg-surface-grouped p-5 sm:p-7">
          <div className="flex flex-wrap items-center justify-end gap-3">
            <MapSearchControl id="map-browser-search" value={mapSearchInput} onChange={setMapSearchInput} />
            {mapScope === "community" ? <MapSortControl value={mapSort} onChange={setMapSort} /> : null}
          </div>

          {mapActionNotice ? (
            <LobbyNotice title={mapActionNotice.title} tone="success" className="mt-5 rounded-2xl">
              {mapActionNotice.message}
            </LobbyNotice>
          ) : null}

          {mapScope === "mine" && !canUploadCustomMaps ? (
            <LobbyPlaceholder className="mt-6">Sign in to create custom maps.</LobbyPlaceholder>
          ) : mapsLoading ? (
            <CenteredSpinner label="Loading maps" className="mt-8" />
          ) : readyMaps.length === 0 ? (
            <LobbyPlaceholder className="mt-8 p-8">
              {hasMapSearch ? "No maps match your search." : "No maps in this section yet."}
            </LobbyPlaceholder>
          ) : (
            <div className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {readyMaps.map((item) => (
                <div key={item.id} className="overflow-hidden rounded-xl">
                  <MapCard
                    item={item}
                    mode={partyActive ? "select" : "link"}
                    thumbnailURL={thumbnailURL}
                    onSelect={selectMapForParty}
                  />
                </div>
              ))}
            </div>
          )}

          {mapScope === "mine" && canUploadCustomMaps ? (
            <SectionCard className="mt-7 rounded-xl p-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <LobbySectionHeader title="Upload Map" description="Create a custom map from a JSON file." />
                <ButtonLink href="/maps/upload" variant="primary" icon={<Upload size={17} />}>
                  Upload
                </ButtonLink>
              </div>
            </SectionCard>
          ) : null}
        </section>
      </div>
    </AppPanel>
  );
}

export function MapUploadPanel({
  canUploadCustomMaps,
  mapUploadForm,
}: {
  canUploadCustomMaps: boolean;
  mapUploadForm: React.ReactNode;
}) {
  return (
    <AppPanel className="rounded-xl p-4 sm:p-6">
      <div className="space-y-5">
        <BackToMapsLink />
        <SectionCard className="rounded-xl p-4 sm:p-5">
          <div className="mb-5">
            <LobbySectionHeader
              eyebrow="Upload Map"
              title="Create a Custom Map"
              description="Use external sites (e.g., map-making.app) to create your own maps. Upload a JSON file to play your custom map in GeoDuels."
            />
          </div>
          {canUploadCustomMaps ? mapUploadForm : <LobbyPlaceholder>Sign in to create custom maps.</LobbyPlaceholder>}
        </SectionCard>
      </div>
    </AppPanel>
  );
}

function BackToMapsLink() {
  return (
    <ButtonLink href="/maps" variant="secondary" size="sm" icon={<ArrowLeft size={16} />}>
      Back
    </ButtonLink>
  );
}

export function MapDetailsPanel({
  accessToken,
  canInteractWithMaps,
  canUploadCustomMaps,
  commentBody,
  commentComposerFocused,
  createCommentPending,
  displayName,
  expandedCommentIds,
  favoriteMap,
  isAdmin,
  isModerator,
  mapActionNotice,
  mapPickerFlow,
  onCancelComment,
  onDeleteComment,
  onDeleteMap,
  onPostComment,
  onPostReply,
  onPublishMap,
  onUpdateMap,
  onLocationFile,
  onSetMapOfficial,
  onSetMapRole,
  onSetCommentBody,
  onSetCommentComposerFocused,
  onSetOpenCommentMenuId,
  onSetReplyBody,
  onSetReplyToCommentId,
  onToggleCommentLike,
  onToggleCommentReplies,
  openCommentMenuId,
  playMapSingleplayer,
  replyBody,
  replyToCommentId,
  selectMapForParty,
  selectedMapDetails,
  selectedMapLoading,
  singleplayerDisabled,
  thumbnailURL,
  userAvatar,
  userAvatarFallback,
  userEmail,
  userId,
}: {
  accessToken: string;
  canInteractWithMaps: boolean;
  canUploadCustomMaps: boolean;
  commentBody: string;
  commentComposerFocused: boolean;
  createCommentPending: boolean;
  displayName: string;
  expandedCommentIds: Record<string, boolean>;
  favoriteMap: (input: { mapId: string; favorite: boolean }) => void;
  isAdmin: boolean;
  isModerator: boolean;
  mapActionNotice: MapActionNotice | null;
  mapPickerFlow: boolean;
  onCancelComment: () => void;
  onDeleteComment: (commentId: string) => void;
  onDeleteMap: (map: CustomMap) => void;
  onPostComment: () => void;
  onPostReply: (commentId: string) => void;
  onPublishMap: (mapId: string) => void;
  onUpdateMap: (mapId: string, input: MapUpdateInput) => Promise<CustomMap>;
  onLocationFile: (mapId: string, file: File) => void;
  onSetMapOfficial: (mapId: string, official: boolean) => void;
  onSetMapRole: (mapId: string, role: GameplayMapRole) => void;
  onSetCommentBody: (body: string) => void;
  onSetCommentComposerFocused: (focused: boolean) => void;
  onSetOpenCommentMenuId: (commentId: string) => void;
  onSetReplyBody: (body: string) => void;
  onSetReplyToCommentId: (commentId: string) => void;
  onToggleCommentLike: (commentId: string, liked: boolean) => void;
  onToggleCommentReplies: (commentId: string) => void;
  openCommentMenuId: string;
  playMapSingleplayer: (item: CustomMap) => void;
  replyBody: string;
  replyToCommentId: string;
  selectMapForParty: (item: CustomMap) => void;
  selectedMapDetails: MapDetails | undefined;
  selectedMapLoading: boolean;
  singleplayerDisabled: boolean;
  thumbnailURL: (item: Pick<CustomMap, "thumbnailVariant" | "thumbnailKey">) => string;
  userAvatar?: string;
  userAvatarFallback: string;
  userEmail: string;
  userId: string;
}) {
  const [editMetadataOpen, setEditMetadataOpen] = useState(false);

  if (selectedMapLoading || !selectedMapDetails) {
    return (
      <AppPanel className="rounded-2xl p-4 sm:p-6">
        <CenteredSpinner label="Loading map details" />
      </AppPanel>
    );
  }

  const map = selectedMapDetails.map;

  return (
    <AppPanel className="rounded-2xl p-4 sm:p-6">
      <div className="space-y-5">
        <div className="flex items-center justify-between gap-3">
          <BackToMapsLink />
          {canInteractWithMaps ? (
            <Button
              type="button"
              variant="secondary"
              size="icon"
              onClick={() => favoriteMap({ mapId: map.id, favorite: !map.favorited })}
              aria-label={map.favorited ? "Remove saved map" : "Save map"}
              title={map.favorited ? "Saved" : "Save"}
              className={cn(
                "h-11 min-h-11 w-11 rounded-full",
                map.favorited && "border-status-success/40 bg-status-success/15 text-status-success hover:bg-status-success/20",
              )}
            >
              <Star size={18} fill={map.favorited ? "currentColor" : "none"} />
            </Button>
          ) : null}
        </div>
        {mapActionNotice ? (
          <LobbyNotice title={mapActionNotice.title} tone="success" className="rounded-2xl">
            {mapActionNotice.message}
          </LobbyNotice>
        ) : null}
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]">
          <section
            className="relative min-h-[280px] overflow-hidden rounded-xl bg-cover bg-center"
            style={{ backgroundImage: `url(${thumbnailURL(map)})` }}
          >
            <div className="absolute inset-0 bg-scrim" />
            <div className="relative flex min-h-[280px] flex-col justify-end p-5 sm:p-6">
              <div className="max-w-[720px]">
                <h2 className="text-display-md font-strong leading-heading tracking-heading text-content-primary">{map.displayName}</h2>
                <div className="mt-3 flex flex-wrap items-center gap-3 text-body-sm font-strong text-content-primary">
                  <span>
                    By{" "}
                    <PlayerProfileLink userId={map.ownerUserId} nickname={map.authorName} disabled={!map.ownerUserId} className="hover:text-status-success hover:underline">
                      {map.authorName || "GeoDuels"}
                    </PlayerProfileLink>
                  </span>
                  <MapDifficulty difficulty={map.difficulty} />
                </div>
              </div>
            </div>
          </section>

          <SectionCard className="rounded-xl p-4 sm:p-5">
            <div className="grid gap-3">
              {[
                { label: "plays", value: map.playCount, icon: <Play size={20} fill="currentColor" /> },
                { label: "locations", value: map.locationCount, icon: <MapIcon size={20} /> },
                { label: "favorites", value: map.favoriteCount, icon: <Heart size={20} /> },
              ].map((metric) => (
                <div key={metric.label} className="grid grid-cols-[64px_minmax(0,1fr)] overflow-hidden rounded-lg border border-border-default bg-surface-grouped">
                  <div className="flex items-center justify-center bg-surface-fill text-status-success">{metric.icon}</div>
                  <div className="px-4 py-3">
                    <div className="text-heading-md font-strong leading-collapsed text-content-primary">{metric.value.toLocaleString()}</div>
                    <div className="mt-1 text-label font-strong lowercase text-content-secondary">{metric.label}</div>
                  </div>
                </div>
              ))}
            </div>
            <p className="mt-4 text-body-sm font-medium leading-body text-content-secondary">
              {map.description || "No description has been added for this map yet."}
            </p>
          </SectionCard>
        </div>

        <SectionCard className="flex flex-col gap-3 rounded-2xl p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap gap-2">
            <Badge size="md">Infinite Clock</Badge>
          </div>
          <div className="flex flex-wrap gap-2">
            {mapPickerFlow ? (
              <Button type="button" variant="primary" onClick={() => selectMapForParty(map)} size="lg" className="min-h-[46px] rounded-xl px-6">
                <MapIcon className="mr-2" size={18} />
                Use This Map
              </Button>
            ) : (
              <>
                <LobbyPlaceholder className="flex min-h-[46px] items-center gap-2 border-solid px-3 py-0 text-left text-body-sm font-strong">
                  <Trophy size={17} aria-hidden="true" />
                  <span>
                    <span className="sr-only">Personal best: </span>
                    {map.personalBest ? map.personalBest.score.toLocaleString() : "No PB yet"}
                  </span>
                </LobbyPlaceholder>
                <Button type="button" variant="primary" onClick={() => playMapSingleplayer(map)} disabled={singleplayerDisabled} size="lg" className="min-h-[46px] rounded-xl px-6">
                  <Play className="mr-2" size={18} fill="currentColor" />
                  Play
                </Button>
              </>
            )}
          </div>
        </SectionCard>

        {map.ownerUserId === userId && canUploadCustomMaps ? (
          <SectionCard className="flex flex-col gap-4 rounded-2xl p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="text-body font-strong tracking-heading text-content-primary">Map Options</h3>
            </div>
            <div className="flex flex-wrap gap-2">
              {!map.publishedAt ? (
                <Button type="button" variant="primary" size="sm" onClick={() => onPublishMap(map.id)}>
                  Publish
                </Button>
              ) : null}
              <Button type="button" size="sm" icon={<Pencil size={14} />} onClick={() => setEditMetadataOpen(true)}>Edit</Button>
              <FileInputTrigger
                accept=".json,application/json"
                onChange={(event) => { const file = event.target.files?.[0]; if (file) onLocationFile(map.id, file); event.currentTarget.value = ""; }}
                className="inline-flex min-h-10 items-center rounded-lg border border-border-default bg-surface-fill px-4 text-label font-strong text-content-primary hover:border-border-strong hover:bg-surface-grouped"
              >
                <Upload className="mr-1.5" size={14} /> New Version
              </FileInputTrigger>
              <Button type="button" variant="danger" size="sm" onClick={() => onDeleteMap(map)}>
                Delete
              </Button>
            </div>
          </SectionCard>
        ) : null}

        {isAdmin ? (
          <MapAdminOperations
            map={map}
            onDeleteMap={onDeleteMap}
            onSetOfficial={onSetMapOfficial}
            onSetRole={onSetMapRole}
          />
        ) : null}

        {editMetadataOpen ? (
          <MapEditMetadataModal
            map={map}
            onClose={() => setEditMetadataOpen(false)}
            onSave={onUpdateMap}
          />
        ) : null}

        <MapComments
          accessToken={accessToken}
          canInteractWithMaps={canInteractWithMaps}
          commentBody={commentBody}
          commentComposerFocused={commentComposerFocused}
          comments={selectedMapDetails.comments}
          createCommentPending={createCommentPending}
          displayName={displayName}
          expandedCommentIds={expandedCommentIds}
          isAdmin={isAdmin}
          isModerator={isModerator}
          onCancelComment={onCancelComment}
          onDeleteComment={onDeleteComment}
          onPostComment={onPostComment}
          onPostReply={onPostReply}
          onSetCommentBody={onSetCommentBody}
          onSetCommentComposerFocused={onSetCommentComposerFocused}
          onSetOpenCommentMenuId={onSetOpenCommentMenuId}
          onSetReplyBody={onSetReplyBody}
          onSetReplyToCommentId={onSetReplyToCommentId}
          onToggleCommentLike={onToggleCommentLike}
          onToggleCommentReplies={onToggleCommentReplies}
          openCommentMenuId={openCommentMenuId}
          replyBody={replyBody}
          replyToCommentId={replyToCommentId}
          userAvatar={userAvatar}
          userAvatarFallback={userAvatarFallback}
          userEmail={userEmail}
        />
      </div>
    </AppPanel>
  );
}
