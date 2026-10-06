import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Select } from "../../../components/ui/select";
import { Textarea } from "../../../components/ui/textarea";
import { Heading } from "../../../components/ui/typography";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { warningRequest, type ModerationWarning } from "../../notifications/lib/warnings-client";
import { AdminPanel } from "./admin-primitives";
import { formatDate } from "../lib/admin-format";

export function PlayerWarnings({ config, accessToken, userId, onChanged }: {
  config: RuntimeConfig; accessToken: string; userId: string; onChanged: () => Promise<unknown>;
}) {
  const [category, setCategory] = useState("chat_abuse");
  const [message, setMessage] = useState("");
  const [resetNickname, setResetNickname] = useState(false);
  const [evidenceMessageId, setEvidenceMessageId] = useState("");
  const path = `staff/players/${encodeURIComponent(userId)}/warnings`;
  const query = useQuery({
    queryKey: ["player-warnings", userId, accessToken],
    queryFn: () => warningRequest<{ warnings: ModerationWarning[] }>(config, accessToken, path),
    refetchInterval: 30_000,
  });
  const refresh = async () => { await query.refetch(); await onChanged(); };
  const issue = useMutation({
    mutationFn: () => warningRequest(config, accessToken, path, "POST", { category, message, resetNickname, evidenceMessageId }),
    onSuccess: async () => { setMessage(""); setEvidenceMessageId(""); await refresh(); },
  });
  const withdraw = useMutation({
    mutationFn: (id: number) => warningRequest(config, accessToken, `${path}/${id}/withdraw`, "POST"),
    onSuccess: refresh,
  });
  return (
    <AdminPanel className="space-y-4 p-4">
      <Heading as="h3" variant="heading-sm">Warnings</Heading>
      <form className="space-y-3" onSubmit={event => { event.preventDefault(); issue.mutate(); }}>
        <label className="block text-body-sm">Category
          <Select className="mt-1 w-full" value={category} onChange={event => {
            setCategory(event.target.value);
            setResetNickname(event.target.value === "staff_impersonation" || event.target.value === "inappropriate_nickname");
          }}>
            <option value="chat_abuse">Chat abuse</option>
            <option value="staff_impersonation">Staff impersonation</option>
            <option value="inappropriate_nickname">Inappropriate nickname</option>
            <option value="other">Other</option>
          </Select>
        </label>
        <label className="block text-body-sm">Message shown to the player
          <Textarea className="mt-1 w-full" value={message} onChange={event => setMessage(event.target.value)} required maxLength={1000} rows={3} />
        </label>
        <label className="block text-body-sm">Chat message ID (optional evidence)
          <Input className="mt-1 w-full" value={evidenceMessageId} maxLength={128} onChange={event => setEvidenceMessageId(event.target.value)} />
        </label>
        <label className="flex items-center gap-2 text-body-sm">
          <Input type="checkbox" className="min-h-0 h-4 w-4" checked={resetNickname} onChange={event => setResetNickname(event.target.checked)} />
          Reset nickname to a neutral name
        </label>
        <Button type="submit" disabled={!message.trim() || issue.isPending}>
          {issue.isPending ? "Sending…" : resetNickname ? "Warn & reset nickname" : "Send warning"}
        </Button>
        {issue.isSuccess ? <p className="text-body-sm text-status-success">Warning sent.</p> : null}
      </form>
      {query.isLoading ? <p className="text-body-sm text-content-secondary">Loading warnings…</p> : null}
      {(query.error || issue.error || withdraw.error) ? <p role="alert" className="text-body-sm text-status-danger">{(query.error || issue.error || withdraw.error)?.message}</p> : null}
      <div className="space-y-3">
        {query.data?.warnings.map(w => (
          <div key={w.id} className="space-y-2 rounded-md border border-border-default bg-surface-grouped p-3 text-body-sm">
            <p className="font-semibold">{w.category.replaceAll("_", " ")} · {w.withdrawnAt ? "Withdrawn" : w.acknowledgedAt ? "Acknowledged" : "Awaiting acknowledgment"}</p>
            <p className="whitespace-pre-wrap break-words">{w.message}</p>
            <p className="text-content-secondary">{w.actorName || w.actorUserId || "Staff"} · {formatDate(w.createdAt)}</p>
            {w.acknowledgedAt ? <p className="text-content-secondary">Acknowledged {formatDate(w.acknowledgedAt)}</p> : null}
            {w.resetNickname ? <p className="text-content-secondary">Nickname reset: {w.previousNickname} → {w.resetNickname}</p> : null}
            {w.evidenceMessageId ? <p className="break-all text-content-secondary">Evidence message: {w.evidenceMessageId}</p> : null}
            {!w.withdrawnAt ? <Button variant="ghost" size="sm" disabled={withdraw.isPending} onClick={() => withdraw.mutate(w.id)}>Withdraw warning</Button> : <p className="text-content-secondary">Withdrawn {formatDate(w.withdrawnAt)}</p>}
          </div>
        ))}
        {query.data && !query.data.warnings.length ? <p className="text-body-sm text-content-secondary">No warnings for this player.</p> : null}
      </div>
    </AdminPanel>
  );
}
