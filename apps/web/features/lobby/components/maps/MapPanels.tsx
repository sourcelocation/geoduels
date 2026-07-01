import Link from "next/link";
import React, { useState } from "react";
import { ArrowLeft, ChartNoAxesColumnIncreasing, Check, Heart, Loader2, Map as MapIcon, Pencil, Play, Search, Star, Trophy, Upload, X } from "lucide-react";
import PlayerProfileLink from "../../../../components/ui/PlayerProfileLink";
import { Tooltip } from "../../../../components/ui/Tooltip";
import { cn } from "../../../../lib/cn";
import { toPublicEntityId } from "../../../../lib/entity-id";
import type { CustomMap, GameplayMapRole, MapDetails, MapScope, MapSort, MapUpdateInput } from "../../../maps/lib/maps-client";
import {
  LobbyActionButton,
  LobbyInput,
  LobbyMutedBox,
  LobbyNotice,
  LobbyPanel,
  LobbySectionHeader,
} from "../lobby-primitives";
import { MapAdminOperations } from "./MapAdminOperations";
import { MapComments } from "./MapComments";
import { MapEditMetadataModal } from "./MapEditMetadataModal";

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
      <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#77f0be]" size={16} />
      <LobbyInput
        id={id}
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Search maps"
        className="h-11 w-full rounded-xl py-2 pl-9 pr-10 font-semibold"
      />
      {value ? (
        <button
          type="button"
          aria-label="Clear map search"
          onClick={() => onChange("")}
          className="absolute right-2 top-1/2 inline-flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-full text-[#a9bfd4] transition hover:bg-white/[0.08] hover:text-white"
        >
          <X size={15} />
        </button>
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
        <button
          key={item.scope}
          type="button"
          onClick={() => onChange(item.scope)}
          className={`rounded-[14px] px-4 py-3 text-left text-sm font-extrabold transition ${
            value === item.scope ? "bg-accentPrimary text-white" : "bg-white/[0.05] text-[#a9bfd4] hover:bg-white/[0.09]"
          }`}
        >
          {item.label}
        </button>
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
    <div className="flex rounded-[14px] border border-white/10 bg-black/20 p-1">
      {(["trending", "popular", "new"] as MapSort[]).map((sort) => (
        <button
          key={sort}
          type="button"
          onClick={() => onChange(sort)}
          className={`rounded-[10px] px-3 py-2 text-[11px] font-black uppercase tracking-[0.08em] ${
            value === sort ? "bg-white text-[#10201a]" : "text-[#a9bfd4] hover:bg-white/[0.08]"
          }`}
        >
          {sort === "popular" ? "Most Popular" : sort}
        </button>
      ))}
    </div>
  );
}

function formatMapMetric(value: number) {
  if (value < 1000) return value.toLocaleString();
  return `${Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: value < 10000 ? 1 : 0 }).format(value)}+`;
}

const difficultyTone = { easy: "text-[#4ade80]", normal: "text-[#facc15]", hard: "text-[#fb6a4a]" } satisfies Record<CustomMap["difficulty"], string>;

export function MapDifficulty({
  difficulty,
  className,
}: {
  difficulty: CustomMap["difficulty"];
  className?: string;
}) {
  return (
    <span
      className={cn("inline-flex items-center gap-1.5 text-[12px] font-extrabold uppercase text-[#c7dde1]", className)}
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
    <div
      className={cn(
        "group relative aspect-[16/9] overflow-hidden rounded-2xl bg-[#062722] text-left",
        "transition duration-200 hover:-translate-y-0.5",
        selected && "ring-2 ring-accentPrimary",
      )}
    >
      <img
        src={thumbnailURL(item)}
        alt=""
        className="absolute inset-0 h-full w-full object-cover opacity-85 transition duration-500 group-hover:scale-[1.035]"
      />
      <div className="absolute inset-0 bg-gradient-to-b from-[#053f3d]/10 via-[#062722]/25 to-[#031817]/88" />
      <div className="absolute inset-0 bg-gradient-to-r from-black/20 via-transparent to-black/25" />
      <div className="absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-black/70 via-black/28 to-transparent" />

      <div className="absolute right-3 top-3 flex flex-wrap justify-end gap-2">
        {item.system || item.official ? (
          <Tooltip content="Official map">
            <span className="inline-flex h-6 w-6 items-center justify-center text-white" aria-label="Official map">
              <Check size={14} strokeWidth={2.5} fill="none" aria-hidden="true" />
            </span>
          </Tooltip>
        ) : null}
        {item.rankedMoving || item.rankedNmpz ? (
          <Tooltip content="Ranked map">
            <span className="inline-flex h-6 w-6 items-center justify-center text-white" aria-label="Ranked map">
              <Trophy size={13} fill="none" strokeWidth={2.25} aria-hidden="true" />
            </span>
          </Tooltip>
        ) : null}
        {selected ? (
          <span className="rounded-full bg-accentPrimary px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.08em] text-white shadow-[0_8px_18px_rgba(42,209,143,0.28)]">
            Selected
          </span>
        ) : null}
      </div>

      <div className="absolute inset-x-0 bottom-0 grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3 p-4">
        <div className="min-w-0">
          <h3 className="truncate text-[18px] font-black leading-tight text-white sm:text-[20px]">{item.displayName}</h3>
          <p className="mt-1.5 truncate text-xs font-bold text-[#c7dde1]">{item.authorName || "GeoDuels"}</p>
        </div>
        <div className="grid gap-1.5 pb-0.5 text-[12px] font-extrabold text-[#c7dde1]">
          <span className="inline-flex items-center justify-end gap-1.5" title="Locations">
            <MapIcon className="text-[#c7dde1]" size={15} />
            {formatMapMetric(item.locationCount)}
          </span>
          <span className="inline-flex items-center justify-end gap-1.5" title="Plays">
            <Play className="text-[#c7dde1]" size={15} />
            {formatMapMetric(item.playCount)}
          </span>
          <MapDifficulty difficulty={item.difficulty} className="justify-end" />
        </div>
      </div>
    </div>
  );

  if (mode === "select") {
    return (
      <button type="button" onClick={() => onSelect?.(item)} className="block w-full text-left">
        {content}
      </button>
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
    <LobbyPanel className="overflow-hidden rounded-3xl">
      <div className="grid lg:min-h-[min(640px,calc(100dvh-11rem))] lg:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="border-b border-white/10 bg-black/20 p-4 lg:border-b-0 lg:border-r">
          <div className="mb-4 flex items-center gap-2 text-white">
            <MapIcon className="text-[#77f0be]" size={22} />
            <span className="text-lg font-black">Maps</span>
          </div>
          <MapScopeNav labels={mapScopeLabels} value={mapScope} onChange={setMapScope} />
        </aside>
        <section className="p-5 sm:p-7">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <LobbySectionHeader
              eyebrow={partyActive ? "Party Map Select" : "Map Browser"}
              title={mapScopeLabels.find((item) => item.scope === mapScope)?.label}
            />
            <div className="flex w-full flex-col gap-3 sm:w-auto sm:items-end">
              <MapSearchControl id="map-browser-search" value={mapSearchInput} onChange={setMapSearchInput} />
              {mapScope === "community" ? <MapSortControl value={mapSort} onChange={setMapSort} /> : null}
            </div>
          </div>

          {mapActionNotice ? (
            <LobbyNotice title={mapActionNotice.title} tone="success" className="mt-5 rounded-2xl">
              {mapActionNotice.message}
            </LobbyNotice>
          ) : null}

          {mapScope === "mine" && !canUploadCustomMaps ? (
            <LobbyMutedBox className="mt-6">Sign in with a permanent account to create custom maps.</LobbyMutedBox>
          ) : mapsLoading ? (
            <div className="mt-8 flex items-center gap-3 text-sm text-[#a9bfd4]">
              <Loader2 className="animate-spin" size={18} /> Loading maps...
            </div>
          ) : readyMaps.length === 0 ? (
            <LobbyMutedBox className="mt-8 border-dashed p-8 text-center">
              {hasMapSearch ? "No maps match your search." : "No maps in this section yet."}
            </LobbyMutedBox>
          ) : (
            <div className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {readyMaps.map((item) => (
                <div key={item.id} className="overflow-hidden rounded-2xl">
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
            <LobbyPanel variant="subtle" className="mt-7 p-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <LobbySectionHeader
                  eyebrow="Upload Map"
                  title={<span className="text-[18px]">Create a custom map</span>}
                  description="Create a custom map from a JSON file."
                />
                <Link href="/maps/upload" className="inline-flex min-h-[42px] items-center justify-center rounded-[12px] bg-accentPrimary px-4 text-sm font-extrabold uppercase tracking-[0.08em] text-white transition hover:bg-accentPrimaryDeep">
                  <Upload className="mr-2" size={17} />
                  Upload
                </Link>
              </div>
            </LobbyPanel>
          ) : null}
        </section>
      </div>
    </LobbyPanel>
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
    <LobbyPanel className="p-4 sm:p-6">
      <div className="space-y-5">
        <BackToMapsLink />
        <LobbyPanel variant="subtle" className="p-4 sm:p-5">
          <div className="mb-5">
            <LobbySectionHeader
              eyebrow="Upload Map"
              title="Create a Custom Map"
              description="Use external sites (e.g., map-making.app) to create your own maps. Upload a JSON file to play your custom map in GeoDuels."
            />
          </div>
          {canUploadCustomMaps ? mapUploadForm : <LobbyMutedBox>Sign in with a permanent account to create custom maps.</LobbyMutedBox>}
        </LobbyPanel>
      </div>
    </LobbyPanel>
  );
}

function BackToMapsLink() {
  return (
    <Link href="/maps" className="inline-flex min-h-[38px] items-center gap-2 rounded-full border border-white/10 bg-white/[0.08] px-4 text-[12px] font-extrabold uppercase tracking-[0.08em] text-[#d6e4ed] transition hover:bg-white/[0.12] hover:text-white">
      <ArrowLeft size={16} />
      Back
    </Link>
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
      <LobbyPanel className="p-4 sm:p-6">
        <div className="flex items-center gap-3 text-sm text-[#a9bfd4]">
          <Loader2 className="animate-spin" size={18} /> Loading map details...
        </div>
      </LobbyPanel>
    );
  }

  const map = selectedMapDetails.map;

  return (
    <LobbyPanel className="p-4 sm:p-6">
      <div className="space-y-5">
        <div className="flex items-center justify-between gap-3">
          <BackToMapsLink />
          {canInteractWithMaps ? (
            <LobbyActionButton
              type="button"
              variant="secondary"
              size="icon"
              onClick={() => favoriteMap({ mapId: map.id, favorite: !map.favorited })}
              aria-label={map.favorited ? "Remove saved map" : "Save map"}
              title={map.favorited ? "Saved" : "Save"}
              className={cn(
                "h-11 min-h-11 w-11 rounded-full",
                map.favorited && "border-accentPrimary/40 bg-accentPrimary/15 text-accentPrimary hover:bg-accentPrimary/20",
              )}
            >
              <Star size={18} fill={map.favorited ? "currentColor" : "none"} />
            </LobbyActionButton>
          ) : null}
        </div>
        {mapActionNotice ? (
          <LobbyNotice title={mapActionNotice.title} tone="success" className="rounded-2xl">
            {mapActionNotice.message}
          </LobbyNotice>
        ) : null}
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]">
          <section
            className="relative min-h-[280px] overflow-hidden rounded-[18px] bg-cover bg-center"
            style={{ backgroundImage: `url(${thumbnailURL(map)})` }}
          >
            <div className="absolute inset-0 bg-black/40" />
            <div className="relative flex min-h-[280px] flex-col justify-end p-5 sm:p-6">
              <div className="max-w-[720px]">
                <h2 className="text-[28px] font-extrabold leading-tight tracking-tight text-white sm:text-[36px]">{map.displayName}</h2>
                <div className="mt-3 flex flex-wrap items-center gap-3 text-sm font-bold text-[#d7e5ee]">
                  <span>
                    By{" "}
                    <PlayerProfileLink userId={map.ownerUserId} nickname={map.authorName} disabled={!map.ownerUserId} className="hover:text-emerald-200 hover:underline">
                      {map.authorName || "GeoDuels"}
                    </PlayerProfileLink>
                  </span>
                  <MapDifficulty difficulty={map.difficulty} />
                </div>
              </div>
            </div>
          </section>

          <LobbyPanel variant="subtle" className="p-4 sm:p-5">
            <div className="grid gap-3">
              {[
                { label: "plays", value: map.playCount, icon: <Play size={20} fill="currentColor" /> },
                { label: "locations", value: map.locationCount, icon: <MapIcon size={20} /> },
                { label: "favorites", value: map.favoriteCount, icon: <Heart size={20} /> },
              ].map((metric) => (
                <div key={metric.label} className="grid grid-cols-[64px_minmax(0,1fr)] overflow-hidden rounded-[12px] border border-white/10 bg-black/25">
                  <div className="flex items-center justify-center bg-white/[0.06] text-[#77f0be]">{metric.icon}</div>
                  <div className="px-4 py-3">
                    <div className="text-[21px] font-extrabold leading-none text-white">{metric.value.toLocaleString()}</div>
                    <div className="mt-1 text-[12px] font-bold lowercase text-[#a9bfd4]">{metric.label}</div>
                  </div>
                </div>
              ))}
            </div>
            <p className="mt-4 text-[14px] font-medium leading-6 text-[#a9bfd4]">
              {map.description || "No description has been added for this map yet."}
            </p>
          </LobbyPanel>
        </div>

        <LobbyPanel variant="subtle" className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap gap-2">
            <span className="rounded-[12px] bg-white/[0.06] px-3 py-2 text-xs font-black uppercase tracking-[0.08em] text-[#a9bfd4]">Moving</span>
            <span className="rounded-[12px] bg-white/[0.06] px-3 py-2 text-xs font-black uppercase tracking-[0.08em] text-[#a9bfd4]">Infinite Clock</span>
          </div>
          <div className="flex flex-wrap gap-2">
            {mapPickerFlow ? (
              <LobbyActionButton type="button" onClick={() => selectMapForParty(map)} size="lg" className="min-h-[46px] rounded-xl px-6">
                <MapIcon className="mr-2" size={18} />
                Use This Map
              </LobbyActionButton>
            ) : (
              <>
                <LobbyMutedBox className="flex min-h-[46px] items-center gap-2 px-3 py-0 text-sm font-extrabold">
                  <Trophy size={17} aria-hidden="true" />
                  <span>
                    <span className="sr-only">Personal best: </span>
                    {map.personalBest ? map.personalBest.score.toLocaleString() : "No PB yet"}
                  </span>
                </LobbyMutedBox>
                <LobbyActionButton type="button" onClick={() => playMapSingleplayer(map)} disabled={singleplayerDisabled} size="lg" className="min-h-[46px] rounded-xl px-6">
                  <Play className="mr-2" size={18} fill="currentColor" />
                  Play
                </LobbyActionButton>
              </>
            )}
          </div>
        </LobbyPanel>

        {map.ownerUserId === userId && canUploadCustomMaps ? (
          <LobbyPanel variant="subtle" className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="text-[16px] font-extrabold tracking-tight text-white">Map Options</h3>
            </div>
            <div className="flex flex-wrap gap-2">
              {!map.publishedAt ? (
                <button type="button" onClick={() => onPublishMap(map.id)} className="min-h-[42px] rounded-[14px] border border-[#77f0be]/20 bg-[#77f0be]/10 px-4 text-xs font-black uppercase tracking-[0.08em] text-white">
                  Publish
                </button>
              ) : null}
              <button type="button" onClick={() => setEditMetadataOpen(true)} className="min-h-[42px] rounded-[14px] border border-white/10 bg-white/[0.06] px-4 text-xs font-black uppercase tracking-[0.08em] text-white hover:bg-white/[0.1]">
                <Pencil className="mr-1.5 inline-block align-[-2px]" size={14} /> Edit
              </button>
              <label className="inline-flex min-h-[42px] cursor-pointer items-center rounded-[14px] border border-white/10 bg-white/[0.06] px-4 text-xs font-black uppercase tracking-[0.08em] text-white hover:bg-white/[0.1]">
                <Upload className="mr-1.5" size={14} /> New Version
                <input type="file" accept=".json,application/json" className="hidden" onChange={(event) => { const file = event.target.files?.[0]; if (file) onLocationFile(map.id, file); event.currentTarget.value = ""; }} />
              </label>
              <button type="button" onClick={() => onDeleteMap(map)} className="min-h-[42px] rounded-[14px] border border-red-400/15 bg-red-400/[0.06] px-4 text-xs font-black uppercase tracking-[0.08em] text-red-200 hover:bg-red-400/10">
                Delete
              </button>
            </div>
          </LobbyPanel>
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
    </LobbyPanel>
  );
}
