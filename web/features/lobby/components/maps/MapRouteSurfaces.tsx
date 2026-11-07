import { forwardRef, useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import Router from "next/router";
import type { MatchConfig } from "../../../matchmaking/lib/queue-client";
import {
  createMap,
  validateMapFile,
  type CustomMap,
  type MapScope,
  type MapSort,
  type MapVisibility,
} from "../../../maps/lib/maps-client";
import {
  useFavoriteMap,
  useMapComments,
  useMapDetails,
  useMapList,
  useMapManagement,
  useMapUploadQuota,
} from "../../../maps/lib/map-hooks";
import { mapThumbnailURL } from "../../../maps/lib/map-thumbnails";
import type { PartyMode } from "../../lib/party-client";
import { isMapScope, type LobbyContentRoute } from "../../lib/lobby-ui";
import { MapUploadForm } from "../MapUploadForm";
import {
  MapDetailsPanel,
  MapsPanel,
  MapUploadPanel,
} from "./MapPanels";
import { MapPickerModal } from "./MapPickerModal";
import { useAppNotice } from "../../../../components/ui/AppNotice";

const tabPanelMotion = {
  initial: {
    opacity: 0,
    y: 16,
    scale: 0.97,
  },
  animate: {
    opacity: 1,
    y: 0,
    scale: 1,
  },
  exit: {
    opacity: 0,
    y: 10,
    scale: 0.97,
  },
  transition: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

const mapScopeLabels: Array<{ scope: MapScope; label: string }> = [
  { scope: "official", label: "Official" },
  { scope: "community", label: "Community" },
  { scope: "favorites", label: "Favorites" },
  { scope: "mine", label: "My Maps" },
];

function thumbnailURL(item: Pick<CustomMap, "thumbnailVariant" | "thumbnailKey">) {
  return mapThumbnailURL(item.thumbnailKey, item.thumbnailVariant);
}

function useMapBrowserState() {
  const [mapScope, setMapScope] = useState<MapScope>("community");
  const [mapSort, setMapSort] = useState<MapSort>("trending");
  const [mapSearchInput, setMapSearchInput] = useState("");
  const [debouncedMapSearch, setDebouncedMapSearch] = useState("");

  useEffect(() => {
    const handle = window.setTimeout(() => setDebouncedMapSearch(mapSearchInput.trim()), 500);
    return () => window.clearTimeout(handle);
  }, [mapSearchInput]);

  return {
    debouncedMapSearch,
    mapScope,
    mapSearchInput,
    mapSort,
    setMapScope,
    setMapSearchInput,
    setMapSort,
  };
}

type MapRouteSurfaceProps = {
  accessToken: string;
  canUploadCustomMaps: boolean;
  contentRoute: Extract<LobbyContentRoute, "maps" | "map-details" | "map-upload">;
  createParty: (mode?: PartyMode, config?: MatchConfig) => Promise<boolean>;
  displayName: string;
  isAdmin: boolean;
  isModerator: boolean;
  mapId: string;
  mapPickerFlow: boolean;
  partyActive: boolean;
  savePartyConfig: (patch: MatchConfig) => void;
  singleplayerDisabled: boolean;
  playMapSingleplayer: (map: CustomMap) => void;
  userAvatar?: string;
  userAvatarFallback: string;
  userEmail: string;
  userId: string;
};

export const MapRouteSurface = forwardRef<HTMLDivElement, MapRouteSurfaceProps>(function MapRouteSurface({
  accessToken,
  canUploadCustomMaps,
  contentRoute,
  createParty,
  displayName,
  isAdmin,
  isModerator,
  mapId,
  mapPickerFlow,
  partyActive,
  savePartyConfig,
  singleplayerDisabled,
  playMapSingleplayer,
  userAvatar,
  userAvatarFallback,
  userEmail,
  userId,
}: MapRouteSurfaceProps, ref) {
  const queryClient = useQueryClient();
  const { show } = useAppNotice();
  const noticeError = (fallback: string) => (error: unknown) => {
    show(error instanceof Error ? error.message : fallback);
  };
  const canInteractWithMaps = !!accessToken && canUploadCustomMaps;
  const browser = useMapBrowserState();
  const [mapName, setMapName] = useState("");
  const [mapDescription, setMapDescription] = useState("");
  const [mapDifficulty, setMapDifficulty] = useState<"easy" | "normal" | "hard">("normal");
  const [mapVisibility, setMapVisibility] = useState<MapVisibility>("private");
  const [mapThumbnailKey, setMapThumbnailKey] = useState("generic/variant-1");
  const [mapThumbnailCategory, setMapThumbnailCategory] = useState<"generic" | "continents" | "countries">("generic");
  const [mapThumbnailSearch, setMapThumbnailSearch] = useState("");
  const [mapFile, setMapFile] = useState<File | null>(null);
  const [mapUploadError, setMapUploadError] = useState("");
  const [commentBody, setCommentBody] = useState("");
  const [replyBody, setReplyBody] = useState("");
  const [replyToCommentId, setReplyToCommentId] = useState("");
  const [commentComposerFocused, setCommentComposerFocused] = useState(false);
  const [expandedCommentIds, setExpandedCommentIds] = useState<Record<string, boolean>>({});
  const [openCommentMenuId, setOpenCommentMenuId] = useState("");

  const setMapScope = browser.setMapScope;

  useEffect(() => {
    if (contentRoute !== "maps" || typeof window === "undefined") return;
    const value = new URLSearchParams(window.location.search).get("scope");
    if (isMapScope(value)) setMapScope(value);
  }, [contentRoute, setMapScope]);

  const mapsQuery = useMapList(
    accessToken,
    userId,
    browser.mapScope,
    browser.mapSort,
    browser.debouncedMapSearch,
    { enabled: contentRoute === "maps" },
  );
  const selectedMapQuery = useMapDetails(accessToken, contentRoute === "map-details" ? mapId : "", userId);
  const mapQuotaQuery = useMapUploadQuota(accessToken, canUploadCustomMaps);
  const favoriteMapMutation = useFavoriteMap(accessToken);
  const mapComments = useMapComments(accessToken, contentRoute === "map-details" ? mapId : "");
  const mapManagement = useMapManagement(accessToken, setMapUploadError, mapQuotaQuery.data?.maxMapLocations);
  const createMapMutation = useMutation({
    mutationFn: async () => {
      if (!accessToken) throw new Error("Sign in to upload maps");
      if (!canUploadCustomMaps) throw new Error("Upgrade your guest profile to upload custom maps");
      if (!mapFile) throw new Error("Choose a JSON map file");
      await validateMapFile(mapFile, mapQuotaQuery.data?.maxMapLocations);
      return createMap(accessToken, {
        file: mapFile,
        displayName: mapName,
        description: mapDescription,
        visibility: mapVisibility,
        difficulty: mapDifficulty,
        thumbnailKey: mapThumbnailKey,
      });
    },
    onSuccess: () => {
      setMapName("");
      setMapDescription("");
      setMapFile(null);
      setMapUploadError("");
      setMapDifficulty("normal");
      setMapVisibility("private");
      setMapThumbnailKey("generic/variant-1");
      setMapThumbnailCategory("generic");
      setMapThumbnailSearch("");
      browser.setMapScope("mine");
      void queryClient.invalidateQueries({ queryKey: ["maps"] });
      void queryClient.invalidateQueries({ queryKey: ["map-upload-quota"] });
      void Router.push({ pathname: "/maps", query: { scope: "mine" } });
    },
    onError: (error) => setMapUploadError(error instanceof Error ? error.message : "Map upload failed"),
  });

  const availableMaps = mapsQuery.data || [];
  const readyMaps = availableMaps.filter((item) => item.status === "ready");
  const hasMapSearch = browser.debouncedMapSearch.length > 0;
  const selectMapForParty = (item: CustomMap) => {
    if (mapPickerFlow) {
      savePartyConfig({ mapId: item.id, mapName: item.displayName });
      return;
    }
    void createParty("duel", {
      mapId: item.id,
      mapName: item.displayName,
      ruleset: "moving",
      roundTimerMode: "none",
      pressureTimeLimitMs: 15000,
    });
  };
  const createCommentMutation = mapComments.createComment;
  const deleteCommentMutation = mapComments.deleteComment;
  const likeCommentMutation = mapComments.likeComment;
  const postMapComment = () => {
    if (!canInteractWithMaps) return;
    createCommentMutation.mutate(
      { body: commentBody },
      {
        onSuccess: () => {
          setCommentBody("");
          setCommentComposerFocused(false);
        },
      },
    );
  };
  const postMapReply = (parentId: string) => {
    if (!canInteractWithMaps) return;
    createCommentMutation.mutate(
      { body: replyBody, parentId },
      {
        onSuccess: () => {
          setReplyBody("");
          setReplyToCommentId("");
          setExpandedCommentIds((current) => ({ ...current, [parentId]: true }));
        },
      },
    );
  };
  const deleteMap = (map: CustomMap) => {
    if (!window.confirm(`Delete ${map.displayName}?`)) return;
    mapManagement.archiveMap.mutate(map.id, {
      onSuccess: () => {
        browser.setMapScope("mine");
        void Router.push({ pathname: "/maps", query: { scope: "mine" } });
      },
      onError: noticeError("Could not delete map"),
    });
  };
  const toggleCommentReplies = (commentId: string) => {
    setExpandedCommentIds((current) => ({ ...current, [commentId]: !current[commentId] }));
  };

  if (contentRoute === "map-upload") {
    return (
      <motion.div ref={ref} key="map-upload" {...tabPanelMotion} className="w-full max-w-[1120px] pointer-events-auto">
        <MapUploadPanel
          canUploadCustomMaps={canUploadCustomMaps}
          mapUploadForm={
            <MapUploadForm
              isGuest={!canUploadCustomMaps}
              mapName={mapName}
              setMapName={setMapName}
              mapDescription={mapDescription}
              setMapDescription={setMapDescription}
              mapDifficulty={mapDifficulty}
              setMapDifficulty={setMapDifficulty}
              mapVisibility={mapVisibility}
              setMapVisibility={setMapVisibility}
              mapThumbnailKey={mapThumbnailKey}
              setMapThumbnailKey={setMapThumbnailKey}
              mapThumbnailCategory={mapThumbnailCategory}
              setMapThumbnailCategory={setMapThumbnailCategory}
              mapThumbnailSearch={mapThumbnailSearch}
              setMapThumbnailSearch={setMapThumbnailSearch}
              mapFile={mapFile}
              setMapFile={setMapFile}
              mapUploadError={mapUploadError}
              quota={mapQuotaQuery.data}
              setMapUploadError={setMapUploadError}
              uploadPending={createMapMutation.isPending}
              onUpload={() => createMapMutation.mutate()}
            />
          }
        />
      </motion.div>
    );
  }

  if (contentRoute === "map-details") {
    return (
      <motion.div ref={ref} key="map-details" {...tabPanelMotion} className="w-full max-w-[1120px] pointer-events-auto">
        <MapDetailsPanel
          accessToken={accessToken}
          canInteractWithMaps={canInteractWithMaps}
          canUploadCustomMaps={canUploadCustomMaps}
          commentBody={commentBody}
          commentComposerFocused={commentComposerFocused}
          createCommentPending={createCommentMutation.isPending}
          displayName={displayName}
          expandedCommentIds={expandedCommentIds}
          favoriteMap={(input) => favoriteMapMutation.mutate(input)}
          isAdmin={isAdmin}
          isModerator={isModerator}
          mapPickerFlow={mapPickerFlow}
          onCancelComment={() => {
            setCommentBody("");
            setCommentComposerFocused(false);
          }}
          onDeleteComment={(commentId) => deleteCommentMutation.mutate({ commentId })}
          onDeleteMap={deleteMap}
          onPostComment={postMapComment}
          onPostReply={postMapReply}
          onPublishMap={(itemId) =>
            mapManagement.publishMap.mutate(itemId, { onError: noticeError("Could not publish map") })
          }
          onUpdateMap={async (itemId, input) => {
            return mapManagement.updateMap.mutateAsync({ mapId: itemId, input });
          }}
          onLocationFile={(itemId, file) =>
            mapManagement.replaceLocations.mutate(
              { mapId: itemId, file },
              { onError: noticeError("Could not replace map locations") },
            )
          }
          onSetMapOfficial={(itemId, official) =>
            mapManagement.setOfficial.mutate(
              { mapId: itemId, official },
              { onError: noticeError("Could not update official status") },
            )
          }
          onSetMapRole={(itemId, role) =>
            mapManagement.setRole.mutate(
              { mapId: itemId, role },
              { onError: noticeError("Could not set map role") },
            )
          }
          onSetCommentBody={setCommentBody}
          onSetCommentComposerFocused={setCommentComposerFocused}
          onSetOpenCommentMenuId={setOpenCommentMenuId}
          onSetReplyBody={setReplyBody}
          onSetReplyToCommentId={setReplyToCommentId}
          onToggleCommentLike={(commentId, liked) => likeCommentMutation.mutate({ commentId, liked })}
          onToggleCommentReplies={toggleCommentReplies}
          openCommentMenuId={openCommentMenuId}
          playMapSingleplayer={playMapSingleplayer}
          replyBody={replyBody}
          replyToCommentId={replyToCommentId}
          selectMapForParty={selectMapForParty}
          selectedMapDetails={selectedMapQuery.data}
          selectedMapLoading={selectedMapQuery.isLoading}
          singleplayerDisabled={singleplayerDisabled}
          thumbnailURL={thumbnailURL}
          userAvatar={userAvatar}
          userAvatarFallback={userAvatarFallback}
          userEmail={userEmail}
          userId={userId}
        />
      </motion.div>
    );
  }

  return (
    <motion.div ref={ref} key="maps" {...tabPanelMotion} className="w-full max-w-6xl pointer-events-auto">
      <MapsPanel
        canUploadCustomMaps={canUploadCustomMaps}
        hasMapSearch={hasMapSearch}
        mapScope={browser.mapScope}
        mapScopeLabels={mapScopeLabels}
        mapSearchInput={browser.mapSearchInput}
        mapSort={browser.mapSort}
        mapsLoading={mapsQuery.isLoading}
        partyActive={partyActive}
        readyMaps={readyMaps}
        setMapScope={browser.setMapScope}
        setMapSearchInput={browser.setMapSearchInput}
        setMapSort={browser.setMapSort}
        selectMapForParty={selectMapForParty}
        thumbnailURL={thumbnailURL}
      />
    </motion.div>
  );
});

type MapPickerControllerProps = {
  accessToken: string;
  canUploadCustomMaps: boolean;
  partyConfig: MatchConfig;
  onClose: () => void;
  savePartyConfig: (patch: MatchConfig) => void;
  userId: string;
};

export function MapPickerController({
  accessToken,
  canUploadCustomMaps,
  partyConfig,
  onClose,
  savePartyConfig,
  userId,
}: MapPickerControllerProps) {
  const browser = useMapBrowserState();
  const mapsQuery = useMapList(
    accessToken,
    userId,
    browser.mapScope,
    browser.mapSort,
    browser.debouncedMapSearch,
    { enabled: true },
  );
  const readyMaps = (mapsQuery.data || []).filter((item) => item.status === "ready");

  return (
    <MapPickerModal
      canUploadCustomMaps={canUploadCustomMaps}
      hasMapSearch={browser.debouncedMapSearch.length > 0}
      partyConfig={partyConfig}
      mapScope={browser.mapScope}
      mapScopeLabels={mapScopeLabels}
      mapSearchInput={browser.mapSearchInput}
      mapSort={browser.mapSort}
      mapsLoading={mapsQuery.isLoading}
      onClose={onClose}
      readyMaps={readyMaps}
      selectMapForParty={(item) => {
        savePartyConfig({ mapId: item.id, mapName: item.displayName });
        onClose();
      }}
      setMapScope={browser.setMapScope}
      setMapSearchInput={browser.setMapSearchInput}
      setMapSort={browser.setMapSort}
      thumbnailURL={thumbnailURL}
    />
  );
}
