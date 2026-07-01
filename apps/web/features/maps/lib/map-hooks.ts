import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import {
  archiveMap,
  createMapComment,
  deleteMapComment,
  getMap,
  getMapUploadQuota,
  listMaps,
  publishMap,
  setGameplayMapRole,
  setMapCommentLike,
  setMapFavorite,
  setMapOfficial,
  replaceMapLocations,
  updateMap,
  validateMapFile,
  type GameplayMapRole,
  type MapComment,
  type MapDetails,
  type MapUpdateInput,
  type MapSort,
  type MapScope,
} from "./maps-client";

function markCommentDeleted(comment: MapComment, commentId: string): { item: MapComment; changed: boolean } {
  let changed = false;
  const replies = comment.replies?.map((reply) => {
    const updated = markCommentDeleted(reply, commentId);
    changed = changed || updated.changed;
    return updated.item;
  });
  if (comment.id === commentId) {
    changed = true;
    return {
      item: {
        ...comment,
        body: "Comment deleted.",
        status: "deleted",
        replies,
      },
      changed,
    };
  }
  return { item: replies ? { ...comment, replies } : comment, changed };
}

function markMapDetailsCommentDeleted(details: MapDetails | undefined, commentId: string): MapDetails | undefined {
  if (!details) return details;
  let changed = false;
  const comments = details.comments.map((comment) => {
    const updated = markCommentDeleted(comment, commentId);
    changed = changed || updated.changed;
    return updated.item;
  });
  return changed ? { ...details, comments } : details;
}

function updateComment(items: MapComment[], commentId: string, update: (comment: MapComment) => MapComment): MapComment[] {
  return items.map((comment) => {
    if (comment.id === commentId) return update(comment);
    if (!comment.replies?.length) return comment;
    return { ...comment, replies: updateComment(comment.replies, commentId, update) };
  });
}

function updateMapDetailsComment(details: MapDetails | undefined, commentId: string, update: (comment: MapComment) => MapComment): MapDetails | undefined {
  return details ? { ...details, comments: updateComment(details.comments, commentId, update) } : details;
}

export function useMapList(
  config: RuntimeConfig,
  accessToken: string | undefined,
  userId: string,
  scope: MapScope,
  sort: MapSort,
  search = "",
  options: { enabled?: boolean } = {},
) {
  const trimmedSearch = search.trim();
  const enabled = options.enabled ?? true;
  return useQuery({
    queryKey: ["maps", scope, sort, trimmedSearch, userId || "anonymous"],
    queryFn: () => listMaps(config, accessToken, { scope, sort: scope === "community" ? sort : undefined, search: trimmedSearch }),
    enabled: enabled && (scope === "official" || scope === "community" || !!accessToken),
    staleTime: 60_000,
  });
}

export function useMapDetails(config: RuntimeConfig, accessToken: string | undefined, mapId: string, userId = "") {
  return useQuery({
    queryKey: ["map-details", mapId, userId || "anonymous"],
    queryFn: () => getMap(config, accessToken, mapId),
    enabled: !!mapId,
    staleTime: 60_000,
  });
}

export function useMapUploadQuota(config: RuntimeConfig, accessToken: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ["map-upload-quota", accessToken ? "authenticated" : "anonymous"],
    queryFn: () => getMapUploadQuota(config, accessToken || ""),
    enabled: enabled && !!accessToken,
    staleTime: 30_000,
  });
}

export function useFavoriteMap(config: RuntimeConfig, accessToken: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ mapId, favorite }: { mapId: string; favorite: boolean }) =>
      setMapFavorite(config, accessToken || "", mapId, favorite),
    onSuccess: (_item, vars) => {
      void queryClient.invalidateQueries({ queryKey: ["maps"] });
      void queryClient.invalidateQueries({ queryKey: ["map-details", vars.mapId] });
    },
  });
}

export function useMapComments(config: RuntimeConfig, accessToken: string | undefined, mapId: string) {
  const queryClient = useQueryClient();
  return {
    createComment: useMutation({
      mutationFn: (input: { body: string; parentId?: string }) =>
        createMapComment(config, accessToken || "", mapId, input),
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: ["map-details", mapId] });
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
      },
    }),
    deleteComment: useMutation({
      mutationFn: ({ commentId }: { commentId: string }) =>
        deleteMapComment(config, accessToken || "", mapId, commentId),
      onSuccess: (_item, vars) => {
        queryClient.setQueriesData<MapDetails>(
          { queryKey: ["map-details", mapId] },
          (details) => markMapDetailsCommentDeleted(details, vars.commentId),
        );
        void queryClient.invalidateQueries({ queryKey: ["map-details", mapId] });
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
      },
    }),
    likeComment: useMutation({
      mutationFn: ({ commentId, liked }: { commentId: string; liked: boolean }) =>
        setMapCommentLike(config, accessToken || "", mapId, commentId, liked),
      onMutate: async ({ commentId, liked }) => {
        await queryClient.cancelQueries({ queryKey: ["map-details", mapId] });
        const previous = queryClient.getQueriesData<MapDetails>({ queryKey: ["map-details", mapId] });
        queryClient.setQueriesData<MapDetails>(
          { queryKey: ["map-details", mapId] },
          (details) => updateMapDetailsComment(details, commentId, (comment) => ({
            ...comment,
            liked,
            likeCount: Math.max(0, comment.likeCount + (liked ? 1 : -1)),
          })),
        );
        return { previous };
      },
      onError: (_error, _vars, context) => {
        context?.previous.forEach(([key, value]) => queryClient.setQueryData(key, value));
      },
      onSuccess: (item) => {
        queryClient.setQueriesData<MapDetails>(
          { queryKey: ["map-details", mapId] },
          (details) => updateMapDetailsComment(details, item.id, () => item),
        );
      },
      onSettled: () => void queryClient.invalidateQueries({ queryKey: ["map-details", mapId] }),
    }),
  };
}

export function useMapManagement(config: RuntimeConfig, accessToken: string | undefined, onUploadError: (message: string) => void, maxMapLocations = 1_000_000) {
  const queryClient = useQueryClient();
  return {
    archiveMap: useMutation({
      mutationFn: (mapId: string) => archiveMap(config, accessToken || "", mapId),
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["maps"] }),
    }),
    publishMap: useMutation({
      mutationFn: (mapId: string) => publishMap(config, accessToken || "", mapId),
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["maps"] }),
    }),
    updateMap: useMutation({
      mutationFn: ({ mapId, input }: { mapId: string; input: MapUpdateInput }) =>
        updateMap(config, accessToken || "", mapId, input),
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-details"] });
      },
    }),
    setOfficial: useMutation({
      mutationFn: ({ mapId, official }: { mapId: string; official: boolean }) =>
        setMapOfficial(config, accessToken || "", mapId, official),
      onSuccess: (_item, vars) => {
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-details", vars.mapId] });
      },
    }),
    setRole: useMutation({
      mutationFn: ({ mapId, role }: { mapId: string; role: GameplayMapRole }) =>
        setGameplayMapRole(config, accessToken || "", mapId, role),
      onSuccess: (_item, vars) => {
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-details", vars.mapId] });
      },
    }),
    replaceLocations: useMutation({
      mutationFn: async ({ mapId, file }: { mapId: string; file: File }) => {
        await validateMapFile(file, maxMapLocations);
        return replaceMapLocations(config, accessToken || "", mapId, file);
      },
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-upload-quota"] });
      },
      onError: (error) => onUploadError(error instanceof Error ? error.message : "Map replacement failed"),
    }),
  };
}
