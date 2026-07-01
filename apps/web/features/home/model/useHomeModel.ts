import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { RESULT_ANIMATION_CONFIG } from "../../../components/ui/round-result-animation-config";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import type { AuthSessionSnapshot } from "../../auth/session";
import { selectActiveChatConversationId } from "../../chat/lib/chat-scope";
import {
    requestDeleteAccount,
    requestDiscordStart,
    requestGoogleStart,
    requestGuestSession,
    requestLogout,
    requestMe,
    requestMatchReport,
    requestUnlinkAuthProvider,
    requestUserNotifications,
    requestSupportDonation,
    requestSession,
    requestRefreshSession,
    requestUpdateSelectedBadge,
    requestUpdateNickname,
    markUserNotificationRead,
    type UserNotification,
} from "../../auth/lib/auth-client";
import {
    type PartyTeamId,
    type PartyMode,
} from "../../lobby/lib/party-client";
import { getHomeRuntime, startHomeRuntime } from "../state/home-runtime";
import { deriveHomeModel } from "./derive-home-model";
import type { HomeModel } from "./types";
import type { ChatEmote } from "../../../components/ui/types";
import { useLobbyData } from "./useLobbyData";
import {
    fetchResumableSession,
    type MatchConfig,
} from "../../matchmaking/lib/queue-client";

import {
    buildSessionFromAuthResponse,
    clearGoogleAuthParams,
    currentReturnTo,
    getErrorMessage,
    type AuthResponse,
    type GuestVerificationView,
} from "./auth-session-model";

function nicknameValidationError(nickname: string) {
    if (!nickname) return "Please choose a nickname.";
    if (nickname.length < 2 || nickname.length > 14) {
        return "Nickname must be 2–14 characters.";
    }
    if (!/^[A-Za-z0-9._]+$/.test(nickname)) {
        return "Use only letters, numbers, dots, and underscores.";
    }
    if (nickname.includes("..") || nickname.includes("__")) {
        return "Repeated dots or underscores are not allowed.";
    }
    return "";
}

export function useHomeModel(options?: {
    routeMatchId?: string | null;
    routeContext?: "home" | "match";
    backgroundDataEnabled?: boolean;
    partyInviteCode?: string | null;
    onPartyEntered?: (inviteCode: string) => void;
    onPartyLeft?: () => void;
}): HomeModel {
    const config = getRuntimeConfig();
    const runtimeRef = useRef(getHomeRuntime(config));
    const { sessionController, matchController, matchRouteController, gameController, partyController, chatController, sfxController } =
        runtimeRef.current;
    const [guestVerification, setGuestVerification] =
        useState<GuestVerificationView>({
            open: false,
            siteKey: "",
            status: "checking",
            error: "",
            resetKey: 0,
        });
    const guestVerificationResolverRef = useRef<{
        resolve: (token: string) => void;
        reject: (error: Error) => void;
    } | null>(null);
    const routeMatchId = options?.routeMatchId ?? null;
    const routeContext = options?.routeContext ?? "home";
    const backgroundDataEnabled = options?.backgroundDataEnabled ?? true;
    const partyInviteCode = options?.partyInviteCode?.trim().toUpperCase() ?? "";
    const onPartyEntered = options?.onPartyEntered;
    const onPartyLeft = options?.onPartyLeft;
    const isMatchRoute = routeContext === "match";
    const queryClient = useQueryClient();

    const auth = useSyncExternalStore(
        sessionController.subscribe,
        sessionController.getState.bind(sessionController),
        sessionController.getState.bind(sessionController),
    );
    const match = useSyncExternalStore(
        matchController.subscribe,
        matchController.getState.bind(matchController),
        matchController.getState.bind(matchController),
    );
    const matchRoute = useSyncExternalStore(
        matchRouteController.subscribe,
        matchRouteController.getState.bind(matchRouteController),
        matchRouteController.getState.bind(matchRouteController),
    );
    const game = useSyncExternalStore(
        gameController.subscribe,
        gameController.getState.bind(gameController),
        gameController.getState.bind(gameController),
    );
    const partyState = useSyncExternalStore(
        partyController.subscribe,
        partyController.getState.bind(partyController),
        partyController.getState.bind(partyController),
    );
    const chatState = useSyncExternalStore(
        chatController.subscribe,
        chatController.getState.bind(chatController),
        chatController.getState.bind(chatController),
    );
    const lobbyData = useLobbyData({
        config,
        sessionController,
        auth,
        enabled: !isMatchRoute && backgroundDataEnabled,
    });

    const notificationsQuery = useQuery({
        queryKey: ["notifications", auth.userId || "anonymous"],
        enabled: !isMatchRoute && backgroundDataEnabled && !!auth.userId && !!auth.accessToken && !auth.nicknameRequired,
        queryFn: async () => {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: false,
            });
            if (!session?.accessToken) {
                return { notifications: [] };
            }
            return requestUserNotifications(config, session.accessToken);
        },
        refetchInterval: 60_000,
        refetchOnMount: false,
        staleTime: 60_000,
    });

    const resumableSessionQuery = useQuery({
        queryKey: ["session-resumable", auth.userId || "anonymous"],
        enabled: !isMatchRoute && backgroundDataEnabled && !partyInviteCode && !!auth.userId && !auth.nicknameRequired,
        queryFn: async ({ signal }) => {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: false,
            });
            if (!session?.accessToken) {
                return { status: "none" as const };
            }
            return fetchResumableSession(config, session.accessToken, signal);
        },
        refetchOnMount: false,
        staleTime: 60_000,
    });

    const refreshSessionMutation = useMutation({
        mutationFn: () => requestRefreshSession(config),
    });
    const sessionMutation = useMutation({
        mutationFn: () => requestSession(config),
    });
    const guestSessionMutation = useMutation({
        mutationFn: ({ turnstileToken }: { turnstileToken?: string }) =>
            requestGuestSession(config, turnstileToken),
    });
    const updateNicknameMutation = useMutation({
        mutationFn: ({
            accessToken,
            nickname,
        }: {
            accessToken: string;
            nickname: string;
        }) => requestUpdateNickname(config, accessToken, nickname),
    });
    const updateSelectedBadgeMutation = useMutation({
        mutationFn: ({
            accessToken,
            badgeId,
        }: {
            accessToken: string;
            badgeId: string;
        }) => requestUpdateSelectedBadge(config, accessToken, badgeId),
    });
    const googleStartMutation = useMutation({
        mutationFn: ({
            accessToken,
            intent,
            returnTo,
        }: {
            accessToken?: string;
            intent?: "signin" | "link" | "upgrade_guest";
            returnTo?: string;
        }) => requestGoogleStart(config, { accessToken, intent, returnTo }),
    });
    const discordStartMutation = useMutation({
        mutationFn: ({
            accessToken,
            intent,
            returnTo,
        }: {
            accessToken?: string;
            intent?: "signin" | "link" | "upgrade_guest";
            returnTo?: string;
        }) => requestDiscordStart(config, { accessToken, intent, returnTo }),
    });
    const unlinkAuthProviderMutation = useMutation({
        mutationFn: ({
            accessToken,
            provider,
        }: {
            accessToken: string;
            provider: "google" | "discord";
        }) => requestUnlinkAuthProvider(config, accessToken, provider),
    });
    const deleteAccountMutation = useMutation({
        mutationFn: ({ accessToken }: { accessToken: string }) =>
            requestDeleteAccount(config, accessToken),
    });
    const supportDonationMutation = useMutation({
        mutationFn: ({ accessToken }: { accessToken: string }) =>
            requestSupportDonation(config, accessToken),
    });

    async function bootstrapSession() {
        const data = await sessionMutation.mutateAsync();
        if (!data) {
            return null;
        }
        const current = sessionController.getState();
        const nextSession = buildSessionFromAuthResponse(data, {
            userId: current.userId,
            nicknameInput: current.nicknameInput,
        });
        sessionController.applySessionSnapshot(nextSession, {
            userEmail:
                typeof data.user?.email === "string"
                    ? data.user.email
                    : current.userEmail,
            displayName:
                typeof data.user?.display_name === "string" && data.user.display_name
                    ? data.user.display_name
                    : current.displayName,
            userAvatar:
                typeof data.user?.avatar_url === "string"
                    ? data.user.avatar_url
                    : current.userAvatar,
            isGuest:
                typeof data.user?.isGuest === "boolean"
                    ? data.user.isGuest
                    : current.isGuest,
            isAdmin:
                typeof data.user?.isAdmin === "boolean"
                    ? data.user.isAdmin
                    : current.isAdmin,
            isModerator:
                typeof data.user?.isModerator === "boolean"
                    ? data.user.isModerator
                    : current.isModerator,
            leaderboard: current.leaderboard,
            authLoading: false,
            authError: "",
        });
        return nextSession;
    }

    async function refreshSession() {
        const current = sessionController.getState();
        const data = await refreshSessionMutation.mutateAsync();
        if (!data) {
            return null;
        }
        const nextSession = buildSessionFromAuthResponse(data, {
            userId: current.userId,
            nicknameInput: current.nicknameInput,
        });
        sessionController.applySessionSnapshot(nextSession, {
            userEmail:
                typeof data.user?.email === "string"
                    ? data.user.email
                    : current.userEmail,
            displayName:
                typeof data.user?.display_name === "string" && data.user.display_name
                    ? data.user.display_name
                    : current.displayName,
            userAvatar:
                typeof data.user?.avatar_url === "string"
                    ? data.user.avatar_url
                    : current.userAvatar,
            isGuest:
                typeof data.user?.isGuest === "boolean"
                    ? data.user.isGuest
                    : current.isGuest,
            isAdmin:
                typeof data.user?.isAdmin === "boolean"
                    ? data.user.isAdmin
                    : current.isAdmin,
            isModerator:
                typeof data.user?.isModerator === "boolean"
                    ? data.user.isModerator
                    : current.isModerator,
            leaderboard: current.leaderboard,
        });
        return nextSession;
    }

    function requestGuestVerificationToken(): Promise<string> {
        if (!config.turnstileSiteKey) {
            return Promise.resolve("");
        }
        guestVerificationResolverRef.current?.reject(
            new Error("Guest verification restarted."),
        );
        return new Promise((resolve, reject) => {
            guestVerificationResolverRef.current = { resolve, reject };
            setGuestVerification((current) => ({
                open: true,
                siteKey: config.turnstileSiteKey,
                status: "checking",
                error: "",
                resetKey: current.resetKey + 1,
            }));
        });
    }

    function submitGuestVerificationToken(token: string) {
        const resolver = guestVerificationResolverRef.current;
        if (!resolver) return;
        guestVerificationResolverRef.current = null;
        setGuestVerification((current) => ({
            ...current,
            status: "creating",
            error: "",
        }));
        resolver.resolve(token);
    }

    function markGuestVerificationExpired(
        message = "Verification expired. Try again.",
    ) {
        setGuestVerification((current) => ({
            ...current,
            status: "error",
            error: message,
            resetKey: current.resetKey + 1,
        }));
    }

    function cancelGuestVerification() {
        guestVerificationResolverRef.current?.reject(
            new Error("Guest verification cancelled."),
        );
        guestVerificationResolverRef.current = null;
        setGuestVerification((current) => ({
            ...current,
            open: false,
            status: "checking",
            error: "",
        }));
    }

    async function ensurePlayableSession() {
        const currentSession = sessionController.getSessionSnapshot();
        if (currentSession) {
            return currentSession;
        }
        const current = sessionController.getState();
        if (current.nicknameRequired) {
            return null;
        }
        sessionController.setAuthPending({
            authLoading: true,
            authError: "",
            nicknameError: "",
        });
        try {
            const bootstrapped = await sessionController.bootstrapSession();
            if (bootstrapped) {
                sessionController.setAuthPending({ authLoading: false, authError: "" });
                return bootstrapped;
            }
            const turnstileToken = await requestGuestVerificationToken();
            const data = await guestSessionMutation.mutateAsync({ turnstileToken });
            const name = data.suggestedNickname || "Guest";
            const nextSession: AuthSessionSnapshot = {
                userId: data.user?.id || "",
                accessToken: data.accessToken || "",
                nicknameRequired: !!data.nicknameRequired,
                nicknameInput: data.suggestedNickname || name,
            };
            sessionController.applySessionSnapshot(nextSession, {
                displayName: name,
                isGuest:
                    typeof data.user?.isGuest === "boolean" ? data.user.isGuest : true,
                isAdmin:
                    typeof data.user?.isAdmin === "boolean" ? data.user.isAdmin : false,
                isModerator:
                    typeof data.user?.isModerator === "boolean"
                        ? data.user.isModerator
                        : false,
                leaderboard: null,
                nicknameError: "",
                authLoading: false,
                authError: "",
            });
            setGuestVerification((current) => ({
                ...current,
                open: false,
                status: "checking",
                error: "",
            }));
            return nextSession;
        } catch (error) {
            const message = getErrorMessage(error, "Guest login failed");
            if (config.turnstileSiteKey) {
                setGuestVerification((current) => ({
                    ...current,
                    open: false,
                    status: "error",
                    error: message,
                    resetKey: current.resetKey + 1,
                }));
            }
            sessionController.setAuthPending({
                authLoading: false,
                authError: message,
            });
            return null;
        }
    }

    const hadPartyRuntimeRef = useRef(false);
    const lastSingleplayerPBRefreshRef = useRef("");

    useEffect(() => {
        startHomeRuntime(runtimeRef.current);
    }, []);

    useEffect(() => {
        sessionController.setNetworkHandlers({
            bootstrapSession,
            refreshSession,
            getPlayableSession: ensurePlayableSession,
        });
    }, [sessionController]);

    useEffect(() => {
        if (isMatchRoute) {
            sessionController.setAuthPending({ authLoading: false });
            return;
        }
        let cancelled = false;
        if (typeof window !== "undefined") {
            const url = new URL(window.location.href);
            const googleAuth = url.searchParams.get("googleAuth");
            const genericAuth = url.searchParams.get("auth");
            if (googleAuth === "success" || genericAuth === "success") {
                clearGoogleAuthParams(url);
                window.history.replaceState({}, "", url.toString());
                sessionController.setAuthPending({ authLoading: true, authError: "" });
                void (async () => {
                    try {
                        await sessionController.bootstrapSession();
                    } catch {
                        if (cancelled) return;
                        sessionController.setAuthPending({
                            authLoading: false,
                            authError:
                                "Sign-in completed, but restoring the session failed. Please try again.",
                        });
                    }
                })();
                return () => {
                    cancelled = true;
                };
            }
            if (googleAuth === "error" || genericAuth === "error") {
                const errorMessage =
                    url.searchParams.get("googleAuthError") ||
                    url.searchParams.get("authError") ||
                    "Login failed";
                clearGoogleAuthParams(url);
                window.history.replaceState({}, "", url.toString());
                sessionController.setAuthPending({
                    authLoading: false,
                    authError: errorMessage,
                });
            }
        }
        if (sessionController.getState().userId) {
            return;
        }
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        void (async () => {
            try {
                const bootstrapped = await sessionController.bootstrapSession();
                if (!bootstrapped) {
                    sessionController.setAuthPending({ authLoading: false });
                }
            } catch {
                if (cancelled) return;
                sessionController.setAuthPending({ authLoading: false });
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [isMatchRoute, sessionController]);

    useEffect(() => {
        if (isMatchRoute || !partyInviteCode) {
            return;
        }
        void partyController.ensureParty(partyInviteCode);
    }, [
        auth.accessToken,
        auth.userId,
        isMatchRoute,
        partyController,
        partyInviteCode,
    ]);

    useEffect(() => {
        const hasPartyRuntime =
            !!partyState.partyId || !!partyState.inviteCode || !!partyState.snapshot;
        if (
            partyInviteCode &&
            hadPartyRuntimeRef.current &&
            !hasPartyRuntime &&
            partyState.status === "idle"
        ) {
            onPartyLeft?.();
        }
        hadPartyRuntimeRef.current = hasPartyRuntime;
    }, [
        partyInviteCode,
        partyState.inviteCode,
        partyState.partyId,
        partyState.snapshot,
        partyState.status,
        onPartyLeft,
    ]);

    useEffect(() => {
        if (!match.lastFinalizedMatchId) return;
        void queryClient.invalidateQueries({ queryKey: ["me"] });
        void queryClient.invalidateQueries({ queryKey: ["leaderboard"] });
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-details"] });
    }, [match.lastFinalizedMatchId, queryClient]);

    useEffect(() => {
        const snapshot = match.snapshot;
        if (
            !snapshot ||
            snapshot.mode !== "singleplayer" ||
            snapshot.state !== "ended" ||
            snapshot.matchId === lastSingleplayerPBRefreshRef.current
        ) {
            return;
        }
        lastSingleplayerPBRefreshRef.current = snapshot.matchId;
        void queryClient.invalidateQueries({ queryKey: ["maps"] });
        void queryClient.invalidateQueries({ queryKey: ["map-details"] });
    }, [match.snapshot, queryClient]);

    const routeSourcePartyId =
        matchRoute.replacement && "sourcePartyId" in matchRoute.replacement
            ? matchRoute.replacement.sourcePartyId || ""
            : "";
    const routeFallbackChatConversationId =
        isMatchRoute && routeSourcePartyId
            ? `party:${routeSourcePartyId}`
            : isMatchRoute &&
                routeMatchId &&
                matchRoute.historySnapshot &&
                matchRoute.historySnapshot.mode !== "singleplayer"
                ? `match:${routeMatchId}`
                : "";
    const activeChatConversationId = selectActiveChatConversationId({
        userId: auth.userId,
        party: partyState,
        match,
    }) || routeFallbackChatConversationId;

    useEffect(() => {
        chatController.setConversation(
            auth.nicknameRequired ? "" : activeChatConversationId,
            auth.accessToken,
        );
    }, [
        activeChatConversationId,
        auth.accessToken,
        auth.nicknameRequired,
        chatController,
    ]);

    const homeResumeMatchId =
        resumableSessionQuery.data?.status === "match"
            ? resumableSessionQuery.data.matchId
            : "";
    const notifications = notificationsQuery.data?.notifications || [];

    const baseView = deriveHomeModel({
        auth,
        match: {
            ...match,
            onlinePlayers:
                typeof lobbyData.onlinePlayers === "number"
                    ? lobbyData.onlinePlayers
                    : match.onlinePlayers,
        },
        game,
        config,
        homeResumeMatchId,
        routeMatchId,
        leaderboardLoading: lobbyData.leaderboardLoading,
        maintenance: lobbyData.maintenance,
        changelogEyebrow: lobbyData.changelogEyebrow,
        changelogTitle: lobbyData.changelogTitle,
        changelogMarkdown: lobbyData.changelogMarkdown,
        changelogSlug: lobbyData.changelogSlug,
        changelogUpdatedAt: lobbyData.changelogUpdatedAt,
    });
    const partyMember = partyState.snapshot?.members.find(
        (member) => member.userId === auth.userId,
    );
    const partyBusy = [
        "creating",
        "joining",
        "connecting",
        "reconnecting",
        "leaving",
    ].includes(partyState.status);
    const partyStatus =
        partyInviteCode && !isMatchRoute && partyState.status === "idle"
            ? "connecting"
            : partyState.status;
    const view = {
        ...baseView,
        overlays: {
            ...baseView.overlays,
            notifications,
            guestVerification,
        },
        lobby: {
            ...baseView.lobby,
            party: {
                status: partyStatus,
                snapshot: partyState.snapshot,
                inviteCode:
                    partyState.inviteCode ||
                    partyState.snapshot?.inviteCode ||
                    partyInviteCode ||
                    "",
                isMember: !!partyMember,
                isOwner: !!partyState.snapshot && partyState.snapshot.ownerUserId === auth.userId,
                busy: partyBusy,
                error: partyState.error,
            },
        },
        chat: {
            conversationId: chatState.conversationId,
            messages: chatState.messages,
            selfUserId: auth.userId,
            error: chatState.error,
        },
    };

    useEffect(() => {
        if (view.game.uiPhase !== "match_end") {
            gameController.setShowMatchEndPage(false);
            return;
        }
        if (game.resultPhase !== "hp_apply") return;
        const timer = setTimeout(
            () => gameController.setShowMatchEndPage(true),
            RESULT_ANIMATION_CONFIG.timeline.endPageDelayMs,
        );
        return () => clearTimeout(timer);
    }, [
        view.game.uiPhase,
        game.resultPhase,
        match.snapshot?.lastRoundResult?.roundId,
        gameController,
        config,
    ]);

    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.code !== "Space" || event.repeat) return;
            const target = event.target as HTMLElement | null;
            const isTyping =
                target &&
                (target.tagName === "INPUT" ||
                    target.tagName === "TEXTAREA" ||
                    target.tagName === "SELECT" ||
                    target.isContentEditable);
            if (isTyping) return;
            if (!view.game.canFinalizeGuess && !view.game.canAdvanceRound) return;
            event.preventDefault();
            if (view.game.canFinalizeGuess) {
                gameController.finalizeGuess();
                return;
            }
            if (view.game.canAdvanceRound) {
                gameController.advanceRound();
            }
        };
        window.addEventListener("keydown", onKeyDown);
        return () => window.removeEventListener("keydown", onKeyDown);
    }, [view.game.canFinalizeGuess, view.game.canAdvanceRound, gameController]);

    const submitRequiredNickname = async () => {
        const nick = sessionController.getState().nicknameInput.trim();
        const validationError = nicknameValidationError(nick);
        if (validationError) {
            sessionController.setAuthPending({
                nicknameError: validationError,
            });
            return;
        }
        const current = sessionController.getState();
        sessionController.setAuthPending({
            nicknameSaving: true,
            nicknameError: "",
            authError: "",
        });
        try {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: true,
            });
            if (!session) {
                sessionController.clearAuthSession(
                    "Session expired. Please sign in again.",
                );
                throw new Error("Session expired. Please sign in again.");
            }
            const data = await updateNicknameMutation.mutateAsync({
                accessToken: session.accessToken,
                nickname: nick,
            });
            const nextSession: AuthSessionSnapshot = {
                userId: typeof data.user?.id === "string" && data.user.id ? data.user.id : current.userId,
                accessToken: data.accessToken || current.accessToken,
                nicknameRequired: !!data.nicknameRequired,
                authMigrationRequired: !!data.authMigrationRequired,
                recoveryAvailable: !!data.recoveryAvailable,
                linkedProviders: Array.isArray(data.linkedProviders)
                    ? data.linkedProviders.filter((provider: unknown): provider is string => typeof provider === "string")
                    : current.linkedProviders || [],
                canPlay: typeof data.canPlay === "boolean" ? data.canPlay : true,
                nicknameInput: nick,
            };
            sessionController.applySessionSnapshot(nextSession, {
                nicknameSaving: false,
                nicknameError: "",
                leaderboard: current.leaderboard,
                displayName: typeof data.user?.display_name === "string" && data.user.display_name ? data.user.display_name : nick,
                userEmail: typeof data.user?.email === "string" ? data.user.email : current.userEmail,
                userAvatar: typeof data.user?.avatar_url === "string" ? data.user.avatar_url : current.userAvatar,
                isGuest: typeof data.user?.isGuest === "boolean" ? data.user.isGuest : current.isGuest,
                isAdmin: typeof data.user?.isAdmin === "boolean" ? data.user.isAdmin : current.isAdmin,
                isModerator: typeof data.user?.isModerator === "boolean" ? data.user.isModerator : current.isModerator,
            });
        } catch (error) {
            sessionController.setAuthPending({
                nicknameSaving: false,
                nicknameError: getErrorMessage(error, "Failed to save nickname"),
            });
        }
    };

    const submitProfileNickname = async (): Promise<boolean> => {
        const current = sessionController.getState();
        const nick = current.nicknameInput.trim();
        const validationError = nicknameValidationError(nick);
        if (validationError) {
            sessionController.setAuthPending({
                nicknameError: validationError,
            });
            return false;
        }
        if (!current.userId || !current.accessToken) {
            sessionController.setAuthPending({
                nicknameError: "Please sign in again.",
            });
            return false;
        }
        if (current.isGuest) {
            sessionController.setAuthPending({
                nicknameError: "Guest nicknames cannot be changed.",
            });
            return false;
        }
        sessionController.setAuthPending({
            nicknameSaving: true,
            nicknameError: "",
            authError: "",
        });
        try {
            const session = await sessionController.ensureFreshSession();
            if (!session) {
                sessionController.clearAuthSession(
                    "Session expired. Please sign in again.",
                );
                throw new Error("Session expired. Please sign in again.");
            }
            const data = await updateNicknameMutation.mutateAsync({
                accessToken: session.accessToken,
                nickname: nick,
            });
            const nextSession: AuthSessionSnapshot = {
                ...session,
                accessToken:
                    typeof data.accessToken === "string" && data.accessToken
                        ? data.accessToken
                        : session.accessToken,
                nicknameRequired: !!data.nicknameRequired,
                canPlay: typeof data.canPlay === "boolean" ? data.canPlay : true,
                nicknameInput: nick,
            };
            sessionController.applySessionSnapshot(nextSession, {
                displayName:
                    typeof data.user?.display_name === "string" &&
                        data.user.display_name
                        ? data.user.display_name
                        : nick,
                nicknameSaving: false,
                nicknameError: "",
            });
            return true;
        } catch (error) {
            sessionController.setAuthPending({
                nicknameSaving: false,
                nicknameError: getErrorMessage(error, "Failed to save nickname"),
            });
            return false;
        }
    };

    const devLogin = () => ensurePlayableSession();

    const logout = () => {
        void requestLogout(config);
        sessionController.clearAuthSession();
    };

    const deleteAccount = async () => {
        const current = sessionController.getState();
        if (!current.accessToken) {
            sessionController.setAuthPending({
                authError: "Please sign in again.",
            });
            return;
        }
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        try {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: true,
            });
            if (!session?.accessToken) {
                throw new Error("Please sign in again.");
            }
            await deleteAccountMutation.mutateAsync({ accessToken: session.accessToken });
            sessionController.clearAuthSession();
        } catch (error) {
            sessionController.setAuthPending({
                authLoading: false,
                authError: getErrorMessage(error, "Failed to delete account"),
            });
            throw error;
        }
    };

    const triggerGoogleSignIn = async () => {
        if (typeof window === "undefined") return;
        if (
            !config.googleClientId ||
            !sessionController.getState().googleSignInEnabled
        ) {
            return;
        }
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        try {
            const data = await googleStartMutation.mutateAsync({
                intent: "signin",
                returnTo: currentReturnTo(),
            });
            if (!data.authURL) {
                throw new Error("Missing Google auth URL");
            }
            window.location.assign(data.authURL);
        } catch (error) {
            sessionController.setAuthPending({
                authLoading: false,
                authError: getErrorMessage(error, "Failed to start Google sign-in"),
            });
        }
    };

    const triggerDiscordSignIn = async () => {
        if (typeof window === "undefined") return;
        if (!config.discordClientId) {
            return;
        }
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        try {
            const data = await discordStartMutation.mutateAsync({
                intent: "signin",
                returnTo: currentReturnTo(),
            });
            if (!data.authURL) {
                throw new Error("Missing Discord auth URL");
            }
            window.location.assign(data.authURL);
        } catch (error) {
            sessionController.setAuthPending({
                authLoading: false,
                authError: getErrorMessage(error, "Failed to start Discord sign-in"),
            });
        }
    };

    const startProviderIntent = async (
        provider: "google" | "discord",
        intent: "link" | "upgrade_guest",
    ) => {
        if (typeof window === "undefined") return;
        if (provider === "google" && !config.googleClientId) return;
        if (provider === "discord" && !config.discordClientId) return;
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        try {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: true,
            });
            if (!session?.accessToken) {
                throw new Error(
                    intent === "link"
                        ? "Sign in before linking another method."
                        : "Sign in as a guest before saving progress.",
                );
            }
            const mutation =
                provider === "google" ? googleStartMutation : discordStartMutation;
            const data = await mutation.mutateAsync({
                accessToken: session.accessToken,
                intent,
                returnTo: currentReturnTo(),
            });
            if (!data.authURL) {
                throw new Error(`Missing ${provider} auth URL`);
            }
            window.location.assign(data.authURL);
        } catch (error) {
            sessionController.setAuthPending({
                authLoading: false,
                authError: getErrorMessage(
                    error,
                    intent === "link"
                        ? "Failed to link sign-in method"
                        : "Failed to save progress",
                ),
            });
        }
    };

    const linkAuthProvider = (provider: "google" | "discord") =>
        startProviderIntent(provider, "link");

    const upgradeGuestWithProvider = (provider: "google" | "discord") =>
        startProviderIntent(provider, "upgrade_guest");

    const unlinkAuthProvider = async (provider: "google" | "discord") => {
        const current = sessionController.getState();
        if (!current.accessToken) {
            sessionController.setAuthPending({
                authError: "Please sign in again.",
            });
            return;
        }
        sessionController.setAuthPending({ authLoading: true, authError: "" });
        try {
            const session = await sessionController.ensureFreshSession(60_000, {
                allowNicknameRequired: true,
            });
            if (!session?.accessToken) {
                throw new Error("Please sign in again.");
            }
            const data = await unlinkAuthProviderMutation.mutateAsync({
                accessToken: session.accessToken,
                provider,
            });
            const latest = sessionController.getState();
            const nextSession = buildSessionFromAuthResponse(data, {
                userId: latest.userId,
                nicknameInput: latest.nicknameInput,
            });
            sessionController.applySessionSnapshot(nextSession, {
                userEmail:
                    typeof data.user?.email === "string" ? data.user.email : latest.userEmail,
                displayName:
                    typeof data.user?.display_name === "string" && data.user.display_name
                        ? data.user.display_name
                        : latest.displayName,
                userAvatar:
                    typeof data.user?.avatar_url === "string" ? data.user.avatar_url : latest.userAvatar,
                isGuest:
                    typeof data.user?.isGuest === "boolean" ? data.user.isGuest : latest.isGuest,
                isAdmin:
                    typeof data.user?.isAdmin === "boolean" ? data.user.isAdmin : latest.isAdmin,
                isModerator:
                    typeof data.user?.isModerator === "boolean"
                        ? data.user.isModerator
                        : latest.isModerator,
                authLoading: false,
                authError: "",
            });
        } catch (error) {
            sessionController.setAuthPending({
                authLoading: false,
                authError: getErrorMessage(error, "Failed to unlink sign-in method"),
            });
        }
    };

    const createParty = async (mode: PartyMode = "duel", matchConfig?: MatchConfig) => {
        const ok = await partyController.createParty(mode, matchConfig);
        const inviteCode = partyController.getState().inviteCode;
        if (ok && inviteCode) {
            onPartyEntered?.(inviteCode);
        }
        return ok;
    };

    const joinParty = async (requestedInviteCode?: string) => {
        const ok = await partyController.joinParty(requestedInviteCode);
        const inviteCode = partyController.getState().inviteCode;
        if (ok && inviteCode) {
            onPartyEntered?.(inviteCode);
        }
        return ok;
    };

    const leaveParty = async () => {
        const hadParty = !!partyController.getState().partyId;
        await partyController.leaveParty();
        if (hadParty && partyController.getState().status === "idle") {
            onPartyLeft?.();
        }
    };

    const kickPartyMember = async (userId: string) => {
        await partyController.kickMember(userId);
    };

    const transferPartyOwner = async (userId: string) => {
        await partyController.transferOwner(userId);
    };

    const startParty = async () => {
        await partyController.startParty();
    };

    const updatePartySettings = async (matchConfig: MatchConfig, mode?: PartyMode) => {
        await partyController.updateSettings(matchConfig, mode);
    };

    const switchPartyTeam = async (teamId: PartyTeamId) => {
        await partyController.switchTeam(teamId);
    };

    const reportPlayer = async (
        reportedUserId: string,
        category = "cheating",
        reason = "",
    ) => {
        const snapshot = matchController.getState().snapshot;
        const session = await sessionController.ensureFreshSession(60_000);
        if (!session?.accessToken || !snapshot?.matchId || !reportedUserId) {
            throw new Error("Report unavailable");
        }
        await requestMatchReport(
            config,
            session.accessToken,
            snapshot.matchId,
            reportedUserId,
            category,
            reason,
        );
    };

    const selectBadge = async (badgeId: string) => {
        const session = await sessionController.ensureFreshSession(60_000);
        if (!session?.accessToken) {
            sessionController.setAuthPending({ authError: "Please sign in again." });
            return;
        }
        const payload = await updateSelectedBadgeMutation.mutateAsync({
            accessToken: session.accessToken,
            badgeId,
        });
        sessionController.applyBadgeSelection(payload);
        if (badgeId) {
            sfxController.play("select");
        }
    };

    const dismissNotification = async (notificationId: number) => {
        const notification = notifications.find((item) => item.id === notificationId);
        queryClient.setQueryData<{ notifications: UserNotification[] }>(
            ["notifications", auth.userId || "anonymous"],
            (current) => ({
                notifications: (current?.notifications || []).filter(
                    (notification) => notification.id !== notificationId,
                ),
            }),
        );
        if (!auth.accessToken) return;
        await markUserNotificationRead(config, auth.accessToken, notificationId);
        if (notification?.type === "badge_unlocked") {
            const resp = await requestMe(config, auth.accessToken);
            if (resp.ok) {
                sessionController.applyProfileSnapshot(await resp.json());
            }
        }
    };

    const startSupportDonation = async () => {
        const session = await sessionController.ensureFreshSession(60_000);
        if (!session?.accessToken) {
            sessionController.setAuthPending({ authError: "Please sign in again." });
            return;
        }
        const payload = await supportDonationMutation.mutateAsync({
            accessToken: session.accessToken,
        });
        if (payload.donationUrl) {
            window.open(payload.donationUrl, "_blank", "noopener,noreferrer");
        }
    };

    const sendChatMessage = (body: string) => {
        return chatController.sendMessage(body);
    };

    const sendChatEmote = (emote: ChatEmote) => {
        return chatController.sendEmote(emote);
    };

    return {
        view,
        actions: {
            joinQueue: matchController.joinQueue,
            startSingleplayer: matchController.startSingleplayer,
            cancelQueue: matchController.cancelQueue,
            createParty,
            joinParty,
            leaveParty,
            kickPartyMember,
            transferPartyOwner,
            startParty,
            updatePartySettings,
            switchPartyTeam,
            placeGuess: gameController.placeGuess,
            finalizeGuess: gameController.finalizeGuess,
            advanceRound: gameController.advanceRound,
            forfeitMatch: gameController.forfeitMatch,
            leaveGame: gameController.leaveGame,
            sendChatMessage,
            sendChatEmote,
            reportPlayer,
            devLogin,
            triggerGoogleSignIn,
            triggerDiscordSignIn,
            linkAuthProvider,
            upgradeGuestWithProvider,
            unlinkAuthProvider,
            loadLeaderboard: lobbyData.loadLeaderboard,
            clearAuthSession: logout,
            deleteAccount,
            submitRequiredNickname,
            submitProfileNickname,
            selectBadge,
            startSupportDonation,
            setNicknameInput: sessionController.setNicknameInputAndClearError,
            dismissNotification,
            submitGuestVerificationToken,
            markGuestVerificationExpired,
            cancelGuestVerification,
        },
    };
}
