import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, ChevronRight, ExternalLink, MessageCircleOff, ShieldAlert, TimerOff } from "lucide-react";
import { useState } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Textarea } from "../../../components/ui/textarea";
import { toPublicEntityId } from "../../../lib/entity-id";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import {
  requestModeratorClaimTask,
  requestModeratorCheatingBan,
  requestModeratorIncident,
  requestModeratorReleaseTask,
  requestModeratorVerdict,
} from "../lib/moderator-client";
import type { CheatingBanSummary, ModerationIncidentDetail, ModerationTimelineItem } from "../types";
import { ModerationMatchReviewList } from "./ModerationMatchReviewList";
import { AdminMetric as Metric, AdminPanel as Panel } from "./admin-primitives";

export function ModeratorIncidentRoute(props: {
  config: RuntimeConfig;
  accessToken: string;
  incidentId: number;
  refreshAdminData: () => Promise<void>;
}) {
  const [note, setNote] = useState("");
  const [reasonCode, setReasonCode] = useState("manual_review");
  const [confirmCheatingBanOpen, setConfirmCheatingBanOpen] = useState(false);
  const [confirmationText, setConfirmationText] = useState("");
  const [cheatingBanSummary, setCheatingBanSummary] = useState<CheatingBanSummary | null>(null);
  const [lastActionMessage, setLastActionMessage] = useState("");
  const queryClient = useQueryClient();
  const incidentQuery = useQuery({
    queryKey: ["moderator-incident", props.incidentId, props.accessToken],
    enabled: !!props.accessToken && props.incidentId > 0,
    queryFn: () => requestModeratorIncident(props.config, props.accessToken, props.incidentId),
  });
  const detail = incidentQuery.data as ModerationIncidentDetail | undefined;
  const task = detail?.tasks?.find((item) => item.status === "claimed" || item.status === "open") || detail?.tasks?.[0];
  const claimMutation = useMutation({
    mutationFn: (taskId: number) => requestModeratorClaimTask(props.config, props.accessToken, taskId),
    onSuccess: props.refreshAdminData,
  });
  const releaseMutation = useMutation({
    mutationFn: (taskId: number) => requestModeratorReleaseTask(props.config, props.accessToken, taskId),
    onSuccess: props.refreshAdminData,
  });
  const verdictMutation = useMutation({
    mutationFn: (input: { verdict: string; enforcementAction?: string; durationHours?: number }) =>
      requestModeratorVerdict(props.config, props.accessToken, props.incidentId, {
        taskId: task?.id,
        verdict: input.verdict,
        reasonCode,
        note,
        enforcementAction: input.enforcementAction,
        durationHours: input.durationHours,
      }),
    onSuccess: async (updated) => {
      queryClient.setQueryData(["moderator-incident", props.incidentId, props.accessToken], updated);
      setLastActionMessage("Verdict saved.");
      await props.refreshAdminData();
    },
  });
  const cheatingBanMutation = useMutation({
    mutationFn: () =>
      requestModeratorCheatingBan(
        props.config,
        props.accessToken,
        detail?.incident.subjectUserId || "",
        moderationReason(reasonCode, note),
      ),
    onSuccess: async (summary: CheatingBanSummary) => {
      setCheatingBanSummary(summary);
      setConfirmCheatingBanOpen(false);
      setConfirmationText("");
      setLastActionMessage("Cheating ban applied and review updated.");
      await incidentQuery.refetch();
      await props.refreshAdminData();
    },
  });

  if (incidentQuery.isLoading) {
    return <Panel className="p-5 text-slate-300">Loading incident...</Panel>;
  }
  if (!detail?.incident) {
    return <Panel className="p-5 text-slate-300">Incident unavailable.</Panel>;
  }

  const incident = detail.incident;
  const subject = detail.subjectPlayer;
  const latestVerdict = detail.verdicts?.[0];
  const actionPending = verdictMutation.isPending || cheatingBanMutation.isPending;

  return (
    <div className="space-y-4">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div>
          <Link href="/moderator/queue" className="inline-flex items-center gap-2 text-sm font-semibold text-slate-400 hover:text-white">
            Back to queue
          </Link>
          <p className="mt-4 text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Incident #{incident.id}</p>
          <h2 className="mt-1 text-3xl font-black text-white">{incident.subjectName || incident.subjectUserId}</h2>
          <p className="mt-1 break-all text-sm text-slate-400">{incident.reasonCode} · {incident.status}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link
            href={`/players/${encodeURIComponent(subject?.displayName || toPublicEntityId(incident.subjectUserId))}?staff=1`}
            className="inline-flex min-h-10 items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm font-semibold text-sky-300 transition hover:bg-slate-800 hover:text-white"
          >
            Public Profile
            <ExternalLink className="h-4 w-4" />
          </Link>
          {task ? <Button disabled={claimMutation.isPending} onClick={() => void claimMutation.mutateAsync(task.id)}>Claim</Button> : null}
          {task ? <Button disabled={releaseMutation.isPending} onClick={() => void releaseMutation.mutateAsync(task.id)}>Release</Button> : null}
        </div>
      </header>

      <div className="grid gap-3 md:grid-cols-4">
        <Metric label="Severity" value={incident.severity} />
        <Metric label="Evidence" value={incident.evidenceStrength} />
        <Metric label="Signals" value={String(incident.signalCount)} />
        <Metric label="Reporters" value={String(incident.uniqueReporterCount)} />
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="space-y-4">
          <Panel className="p-4">
            <h3 className="font-black text-white">Evidence</h3>
            <div className="mt-3 space-y-2">
              {detail.signals.map((signal) => (
                <div key={signal.id} className="rounded-md border border-slate-800 bg-slate-900/70 p-3 text-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-bold text-white">{signal.reasonCode}</span>
                    <span className="rounded bg-slate-800 px-2 py-0.5 text-xs text-slate-300">{signal.source}</span>
                    <span className="text-slate-400">{signal.severity} / {signal.evidenceStrength}</span>
                  </div>
                  {signal.reporterName || signal.reporterUserId ? (
                    <p className="mt-2 text-slate-400">Reporter: {signal.reporterName || signal.reporterUserId}</p>
                  ) : null}
                  {signal.detectorKey ? <p className="mt-1 text-slate-500">{signal.detectorKey} {signal.detectorVersion || ""}</p> : null}
                  {signal.matchId ? (
                    <Link className="mt-2 inline-flex items-center gap-1 text-sky-300 hover:text-white" href={`/match/${encodeURIComponent(toPublicEntityId(signal.matchId))}`}>
                      Open match
                      <ChevronRight className="h-4 w-4" />
                    </Link>
                  ) : null}
                </div>
              ))}
              {detail.signals.length === 0 ? <p className="text-sm text-slate-400">No signals attached.</p> : null}
            </div>
          </Panel>

          <Panel className="p-4">
            <div className="flex items-center justify-between gap-3">
              <h3 className="font-black text-white">Referenced Matches</h3>
              <span className="text-xs text-slate-500">{detail.matches.length}</span>
            </div>
            <ModerationMatchReviewList config={props.config} accessToken={props.accessToken} matches={detail.matches} />
          </Panel>

          <Panel className="p-4">
            <h3 className="font-black text-white">Audit Timeline</h3>
            <AuditTimeline items={detail.auditLog} />
          </Panel>
        </div>

        <aside className="space-y-4">
          <Panel className="p-4">
            <h3 className="font-black text-white">Subject</h3>
            <div className="mt-3 grid gap-3">
              <Metric label="MMR" value={String(subject?.mmr ?? "-")} />
              <Metric label="Games" value={String(subject?.gamesPlayed ?? "-")} />
              <Metric label="Status" value={subject?.isBanned ? "Banned" : "Active"} />
            </div>
          </Panel>

          <Panel className="p-4">
            <h3 className="font-black text-white">Verdict</h3>
            <div className="mt-3 space-y-3">
              <Input value={reasonCode} onChange={(event) => setReasonCode(event.target.value)} placeholder="reason_code" />
              <Textarea value={note} onChange={(event) => setNote(event.target.value)} className="min-h-24 w-full" placeholder="Internal note" />
              <div className="grid gap-2">
                {latestVerdict ? (
                  <div className="rounded-md border border-slate-700 bg-slate-900/70 p-3 text-sm text-slate-200">
                    <p className="font-bold text-white">Latest verdict: {latestVerdict.verdict}</p>
                    <p className="mt-1 text-slate-400">
                      {latestVerdict.enforcementAction || "No enforcement"} · {new Date(latestVerdict.createdAt).toLocaleString()}
                    </p>
                    {latestVerdict.note ? <p className="mt-1 text-slate-400">{latestVerdict.note}</p> : null}
                  </div>
                ) : null}
                {lastActionMessage ? (
                  <div className="rounded-md border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-100">
                    {lastActionMessage}
                  </div>
                ) : null}
                {verdictMutation.isError ? (
                  <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-100">
                    {(verdictMutation.error as Error).message}
                  </div>
                ) : null}
                {cheatingBanSummary ? (
                  <div className="rounded-md border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-100">
                    Cheating ban applied. {cheatingBanSummary.refunds?.refundsIssued ?? 0} refunds issued for {cheatingBanSummary.refunds?.totalRefunded ?? 0} total MMR.
                  </div>
                ) : null}
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "watch" })}>Watch</Button>
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "inconclusive" })}>Inconclusive</Button>
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "dismissed" })}>Dismiss</Button>
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "confirmed", enforcementAction: "warning" })}>Warning</Button>
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "confirmed", enforcementAction: "chat_mute", durationHours: 24 * 30 })}>
                  <MessageCircleOff className="h-4 w-4" />
                  Chat mute 30d
                </Button>
                <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "confirmed", enforcementAction: "temporary_ban", durationHours: 24 * 30 })}>
                  <TimerOff className="h-4 w-4" />
                  Temp ban 30d
                </Button>
                <Button disabled={actionPending} className="border-red-500/50 bg-red-500/15 text-red-100 hover:bg-red-500/25" onClick={() => void verdictMutation.mutateAsync({ verdict: "confirmed", enforcementAction: "permanent_ban" })}>
                  <Ban className="h-4 w-4" />
                  Permanent ban
                </Button>
                {subject?.isBanned ? (
                  <Button disabled={actionPending} onClick={() => void verdictMutation.mutateAsync({ verdict: "confirmed", enforcementAction: "unban" })}>
                    Unban account
                  </Button>
                ) : null}
                <Button disabled={actionPending} className="border-red-400 bg-red-600 text-white hover:bg-red-500" onClick={() => setConfirmCheatingBanOpen(true)}>
                  <ShieldAlert className="h-4 w-4" />
                  Ban for cheating + refund
                </Button>
              </div>
            </div>
          </Panel>
        </aside>
      </div>
      {confirmCheatingBanOpen ? (
        <AppModalShell title="Confirm Cheating Ban" onClose={() => setConfirmCheatingBanOpen(false)} placement="center" maxWidthClassName="max-w-xl">
          <div className="space-y-4 text-sm text-slate-200">
            <div className="rounded-md border border-red-400/40 bg-red-500/15 p-4">
              <div className="flex items-start gap-3">
                <ShieldAlert className="mt-0.5 h-5 w-5 shrink-0 text-red-200" />
                <div>
                  <p className="font-black text-white">This permanently bans the player and issues eligible ranked ELO refunds.</p>
                  <p className="mt-2 text-red-100">
                    Use this only when cheating is confirmed and ranked outcomes were likely corrupted. Temp bans and chat mutes do not refund.
                  </p>
                </div>
              </div>
            </div>
            <div className="rounded-md border border-slate-700 bg-slate-900/70 p-3">
              <p className="font-semibold text-white">{incident.subjectName || incident.subjectUserId}</p>
              <p className="mt-1 break-all text-slate-400">Reason: {moderationReason(reasonCode, note)}</p>
            </div>
            <label className="block">
              <span className="text-xs font-bold uppercase tracking-[0.14em] text-slate-400">Type BAN AND REFUND to continue</span>
              <Input className="mt-2" value={confirmationText} onChange={(event) => setConfirmationText(event.target.value)} />
            </label>
            {cheatingBanMutation.isError ? (
              <p className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-red-100">
                {(cheatingBanMutation.error as Error).message}
              </p>
            ) : null}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button onClick={() => setConfirmCheatingBanOpen(false)}>Cancel</Button>
              <Button
                className="border-red-400 bg-red-600 text-white hover:bg-red-500"
                disabled={confirmationText !== "BAN AND REFUND" || cheatingBanMutation.isPending}
                onClick={() => void cheatingBanMutation.mutateAsync()}
              >
                Permanently ban and refund
              </Button>
            </div>
          </div>
        </AppModalShell>
      ) : null}
    </div>
  );
}

function moderationReason(reasonCode: string, note: string) {
  const code = reasonCode.trim() || "cheating_confirmed";
  const body = note.trim();
  return body ? `${code}: ${body}` : code;
}

function AuditTimeline({ items }: { items: ModerationTimelineItem[] }) {
  if (items.length === 0) return <p className="mt-3 text-sm text-slate-400">No timeline entries yet.</p>;
  return (
    <div className="mt-3 space-y-2">
      {items.map((item) => (
        <div key={item.id} className="rounded-md border border-slate-800 bg-slate-900/60 p-3 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="font-semibold text-white">{item.eventType}</p>
            <p className="text-xs text-slate-500">{new Date(item.createdAt).toLocaleString()}</p>
          </div>
          <p className="mt-1 text-slate-500">
            {item.actorUserId || "system"}{item.reasonCode ? ` · ${item.reasonCode}` : ""}
          </p>
          {item.body ? <p className="mt-2 text-slate-300">{item.body}</p> : null}
        </div>
      ))}
    </div>
  );
}
