import { Flag } from "lucide-react";
import type { Dispatch, SetStateAction } from "react";
import AppModalShell from "../../../../components/ui/AppModalShell";
import { Button } from "../../../../components/ui/button";
import { Textarea } from "../../../../components/ui/textarea";

type PendingReport = { userId: string; name: string };

type Props = {
  pendingReport: PendingReport;
  onClose: () => void;
  onReportPlayer?: (reportedUserId: string, category?: string, reason?: string) => Promise<void> | void;
  onReported: (userId: string) => void;
  reportBusyUserId: string;
  setReportBusyUserId: Dispatch<SetStateAction<string>>;
  reportCategory: string;
  setReportCategory: Dispatch<SetStateAction<string>>;
  reportReason: string;
  setReportReason: Dispatch<SetStateAction<string>>;
  reportError: string;
  setReportError: Dispatch<SetStateAction<string>>;
};

export default function ReportPlayerDialog({
  pendingReport,
  onClose,
  onReportPlayer,
  onReported,
  reportBusyUserId,
  setReportBusyUserId,
  reportCategory,
  setReportCategory,
  reportReason,
  setReportReason,
  reportError,
  setReportError,
}: Props) {
  return (
    <AppModalShell title="Report Player" onClose={onClose} placement="center" showHeader={false} zIndexClassName="z-dialog" maxWidthClassName="max-w-md">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-full border border-status-danger/35 bg-status-danger/15 text-status-danger"><Flag size={18} /></div>
        <div><p className="text-label font-strong uppercase tracking-eyebrow-strong text-status-danger">Report Player</p><h3 className="mt-1 text-heading-md font-strong">Report {pendingReport.name}?</h3></div>
      </div>
      <p className="mt-4 text-body-sm leading-body text-content-secondary">Please select a reason. Only submit a report if you’re sure it’s warranted. Repeatedly submitting false reports may result in a temporary account suspension.</p>
      <div className="mt-5 grid gap-2">
        {[["cheating", "Cheating"], ["boosting", "Boosting / throwing"], ["harassment", "Harassment"], ["profile", "Offensive profile"], ["other", "Other"]].map(([value, label]) => (
          <Button variant="ghost" key={value} type="button" onClick={() => setReportCategory(value)} className={`min-h-10 rounded-md border px-3 text-left text-body-sm font-strong transition ${reportCategory === value ? "border-status-danger/55 bg-status-danger/25 text-content-on-danger" : "border-border-default bg-surface-fill text-content-secondary hover:bg-surface-raised"}`}>{label}</Button>
        ))}
      </div>
      <Textarea value={reportReason} onChange={(event) => setReportReason(event.target.value)} maxLength={1000} placeholder="Optional details" className="mt-4 min-h-24 w-full resize-none rounded-lg border border-border-default bg-surface-inset px-3 py-2 text-body-sm text-content-primary outline-none placeholder:text-content-secondary focus:border-status-danger/50" />
      {reportError ? <p className="mt-3 text-body-sm font-strong text-status-danger">{reportError}</p> : null}
      <div className="mt-6 flex flex-col gap-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" type="button" disabled={reportBusyUserId === pendingReport.userId} onClick={onClose} className="min-h-11 rounded-lg border border-border-default bg-surface-fill px-4 text-body-sm font-strong text-content-primary transition hover:bg-surface-raised disabled:opacity-60">Cancel</Button>
        <Button variant="danger" type="button" disabled={reportBusyUserId === pendingReport.userId} onClick={async () => {
          if (!onReportPlayer) return;
          setReportError("");
          setReportBusyUserId(pendingReport.userId);
          try {
            await onReportPlayer(pendingReport.userId, reportCategory, reportReason);
            onReported(pendingReport.userId);
            onClose();
          } catch (error) {
            setReportError(error instanceof Error ? error.message : "Failed to send report");
          } finally {
            setReportBusyUserId("");
          }
        }} className="min-h-11 rounded-lg border border-status-danger/35 bg-status-danger/20 px-4 text-body-sm font-strong text-content-on-danger transition hover:bg-status-danger/30 disabled:opacity-60">{reportBusyUserId === pendingReport.userId ? "Sending..." : "Send report"}</Button>
      </div>
    </AppModalShell>
  );
}
