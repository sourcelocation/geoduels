import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, MessageCircle } from "lucide-react";
import ChatPanel from "../../../components/ui/ChatPanel";
import { Button } from "../../../components/ui/button";
import type { ChatMessage } from "../../../components/ui/types";
import { toPublicEntityId } from "../../../lib/entity-id";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { requestAdminMatchChat } from "../lib/admin-client";
import { formatAdminDate } from "../lib/admin-format";
import type { ModerationMatch } from "../types";

export function ModerationMatchReviewList(props: {
  config: RuntimeConfig;
  accessToken: string;
  matches: ModerationMatch[];
}) {
  const [reviewMatchId, setReviewMatchId] = useState("");
  const chatQuery = useQuery({
    queryKey: ["admin-match-chat", reviewMatchId, props.accessToken],
    enabled: !!reviewMatchId && !!props.accessToken,
    queryFn: () => requestAdminMatchChat(props.config, props.accessToken, reviewMatchId),
    staleTime: 30_000,
  });
  const messages = (chatQuery.data?.messages || []) as ChatMessage[];

  return (
    <div className="mt-3 space-y-1.5">
      {props.matches.map((match) => {
        const players = Array.isArray(match.players) ? match.players : [];
        const winner = players.find((player) => player.userId === match.winnerUserId);
        const outcome = match.winnerUserId
          ? `${winner?.displayName || match.winnerUserId} won`
          : match.mode === "singleplayer"
            ? "Completed"
            : match.mode
              ? "Draw"
              : "Result unavailable";
        const reviewing = reviewMatchId === match.matchId;
        return (
          <div key={match.matchId} className="rounded-md border border-slate-800 bg-slate-900/70 p-3 text-sm">
            <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="truncate font-semibold text-white">{match.matchId}</span>
                  <span className={match.winnerUserId ? "font-semibold text-emerald-300" : "font-semibold text-amber-300"}>
                    {outcome}
                  </span>
                </div>
                <p className="mt-0.5 truncate text-xs text-slate-400">
                  {players.map((player) => `${player.displayName} ${player.totalScore} pts / ${player.finalHp} HP`).join(" · ") || "Result details unavailable"}
                </p>
                <p className="mt-1 text-xs text-slate-500">
                  {match.mode || "unknown"} · {match.roundCount} rnd{match.roundCount === 1 ? "" : "s"} · {formatAdminDate(match.endedAt)}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => setReviewMatchId(reviewing ? "" : match.matchId)}>
                  <MessageCircle className="h-4 w-4" />
                  {reviewing ? "Hide chat" : "Review chat"}
                </Button>
                <Link className="inline-flex items-center gap-2 px-2 font-semibold text-sky-300 hover:text-white" href={`/match/${encodeURIComponent(toPublicEntityId(match.matchId))}`}>
                  Open match
                  <ExternalLink className="h-4 w-4" />
                </Link>
              </div>
            </div>
            {reviewing ? (
              <div className="mt-3">
                {chatQuery.isLoading ? <p className="text-slate-400">Loading chat…</p> : null}
                {chatQuery.isError ? <p className="text-red-300">Chat log unavailable.</p> : null}
                {!chatQuery.isLoading && !chatQuery.isError ? (
                  <ChatPanel mode="review" messages={messages} selfUserId="" className="relative w-full" />
                ) : null}
              </div>
            ) : null}
          </div>
        );
      })}
      {props.matches.length === 0 ? <p className="text-sm text-slate-400">No referenced matches.</p> : null}
    </div>
  );
}
