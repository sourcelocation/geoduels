import Link from "next/link";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { useAuthState } from "../../auth/components/AuthProvider";
import { toPublicEntityId } from "../../../lib/entity-id";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { fromLocalDateTime, localDateTime } from "../lib/admin-format";
import { nominations, likeNomination, reschedulePromotion, type Nomination } from "../lib/curation-client";

function PromotionSchedule({ closesAt, onSaved }: { closesAt: string; onSaved: (value: string) => void }) {
  const auth = useAuthState();
  const client = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(() => localDateTime(closesAt));
  const [validationError, setValidationError] = useState("");
  const mutation = useMutation({
    mutationFn: (nextClosesAt: string) => reschedulePromotion(auth.accessToken, {
      expectedClosesAt: closesAt,
      closesAt: nextClosesAt,
    }),
    onSuccess: (_, nextClosesAt) => {
      setEditing(false);
      onSaved(nextClosesAt);
    },
    onSettled: () => client.invalidateQueries({ queryKey: ["motw"] }),
  });

  return (
    <div className="space-y-3 rounded-xl border border-border-default bg-surface-panel p-4">
      <p className="text-label text-content-secondary">Next promotion: {new Date(closesAt).toLocaleString()}</p>
      {editing ? (
        <form className="space-y-3" onSubmit={(event) => {
          event.preventDefault();
          const nextClosesAt = fromLocalDateTime(value);
          if (!nextClosesAt || new Date(nextClosesAt).getTime() <= Date.now()) {
            setValidationError("Choose a promotion time in the future.");
            return;
          }
          setValidationError("");
          mutation.mutate(nextClosesAt);
        }}>
          <label htmlFor="motw-promotion-time" className="block text-body-sm font-strong">Promotion date and time (your local timezone)</label>
          <Input
            id="motw-promotion-time"
            type="datetime-local"
            required
            min={localDateTime(new Date().toISOString())}
            value={value}
            disabled={mutation.isPending}
            aria-describedby="motw-schedule-help"
            onChange={(event) => {
              setValue(event.target.value);
              setValidationError("");
              mutation.reset();
            }}
          />
          <p id="motw-schedule-help" className="text-body-sm text-content-secondary">
            Current nominations and likes are kept. Future promotions repeat every seven days from the new time.
          </p>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" variant="primary" loading={mutation.isPending}>Save promotion time</Button>
            <Button disabled={mutation.isPending} onClick={() => {
              setEditing(false);
              setValue(localDateTime(closesAt));
              setValidationError("");
              mutation.reset();
            }}>Cancel</Button>
          </div>
        </form>
      ) : <Button onClick={() => setEditing(true)}>Reschedule promotion</Button>}
      {validationError || mutation.error ? <p role="alert" className="text-status-danger">{validationError || mutation.error?.message}</p> : null}
    </div>
  );
}

export function MapOfTheWeekRoute() {
  const auth = useAuthState();
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [savedClosesAt, setSavedClosesAt] = useState("");
  const query = useQuery({
    queryKey: ["motw", auth.userId, page],
    queryFn: () => nominations(auth.accessToken, page),
    enabled: auth.isModerator,
    refetchInterval: 30_000,
  });
  const mutation = useMutation({
    mutationFn: (item: Nomination) => likeNomination(auth.accessToken, item.id, !item.liked),
    onSuccess: () => client.invalidateQueries({ queryKey: ["motw"] }),
  });
  const data = query.data;
  const pages = Math.max(1, Math.ceil((data?.total || 0) / (data?.pageSize || 20)));

  return (
    <div className="space-y-5">
      <header>
        <h2 className="text-display-md font-strong">Map of the Week</h2>
        <p className="mt-2 text-content-secondary">Nominate maps from their map page. The most liked map wins each week.</p>
      </header>
      {data && auth.isModerator ? <PromotionSchedule key={`${auth.userId}:${data.closesAt}`} closesAt={data.closesAt} onSaved={setSavedClosesAt} /> : null}
      {savedClosesAt && data?.closesAt && new Date(savedClosesAt).getTime() === new Date(data.closesAt).getTime() ? (
        <p role="status" className="text-content-secondary">Promotion rescheduled to {new Date(savedClosesAt).toLocaleString()}.</p>
      ) : null}
      {query.isLoading ? <p>Loading nominations…</p> : null}
      {query.error || mutation.error ? <p role="alert" className="text-status-danger">{(query.error || mutation.error)?.message}</p> : null}
      {data?.items.map(item => (
        <div key={item.id} className="flex items-center justify-between gap-4 rounded-xl border border-border-default bg-surface-panel p-4">
          <div>
            <Link href={`/maps/${toPublicEntityId(item.mapId)}`} className="font-strong hover:underline">{item.name}</Link>
            <p className="text-body-sm text-content-secondary">{item.authorName}</p>
          </div>
          <Button aria-label={`Like ${item.name}`} aria-pressed={item.liked} disabled={mutation.isPending} variant={item.liked ? "primary" : "secondary"} onClick={() => mutation.mutate(item)}>
            <Heart size={16} fill={item.liked ? "currentColor" : "none"} />{item.likes}
          </Button>
        </div>
      ))}
      {data && !data.items.length ? <p className="text-content-secondary">No nominations yet. Without nominations, the top trending community map is selected.</p> : null}
      <div className="flex items-center gap-3">
        <Button disabled={page <= 1} onClick={() => setPage(page - 1)}>Previous</Button>
        <span>Page {page} of {pages}</span>
        <Button disabled={page >= pages} onClick={() => setPage(page + 1)}>Next</Button>
      </div>
    </div>
  );
}
