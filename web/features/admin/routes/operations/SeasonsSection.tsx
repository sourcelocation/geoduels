import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { Heading } from "../../../../components/ui/typography";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import { formatUTCDate } from "../../lib/admin-format";
import {
  requestAdminRankedSeason,
  requestAdminSetRankedSeasonResetRule,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

export function SeasonsSection(props: OperationsSectionProps) {
  const [monthlyResetDay, setMonthlyResetDay] = useState("1");

  const seasonQuery = useQuery({
    queryKey: ["admin-ranked-season", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminRankedSeason(props.accessToken),
  });

  useEffect(() => {
    if (typeof seasonQuery.data?.monthlyResetDay !== "number") return;
    setMonthlyResetDay(String(seasonQuery.data.monthlyResetDay));
  }, [seasonQuery.data?.monthlyResetDay]);

  const saveSeasonResetRule = useMutation({
    mutationFn: () => requestAdminSetRankedSeasonResetRule(props.accessToken, Number(monthlyResetDay)),
    onSuccess: props.refreshAdminData,
  });

  if (props.leaf !== "seasons") {
    return null;
  }

  return (
    <Panel className="p-4">
      <Heading as="h3" variant="heading-sm">Ranked Season</Heading>
      <p className="mt-2 text-body-sm text-content-secondary">Active: {seasonQuery.data?.activeSeasonId || "loading"}</p>
      <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr]">
        <Input
          type="number"
          min={1}
          max={28}
          value={monthlyResetDay}
          onChange={(event) => setMonthlyResetDay(event.target.value)}
          placeholder="Reset day"
        />
        <div className="rounded-lg border border-border-default bg-surface-inset px-3 py-2 text-body-sm text-content-secondary">
          <p>Monthly on day {seasonQuery.data?.monthlyResetDay || "--"} at 21:00 UTC</p>
          <p className="mt-1 text-body-sm text-content-secondary">
            Next reset: {seasonQuery.data?.nextResetAt ? formatUTCDate(seasonQuery.data.nextResetAt) : "Not scheduled"}
          </p>
        </div>
      </div>
      <div className="mt-4">
        <Button disabled={Number(monthlyResetDay) < 1 || Number(monthlyResetDay) > 28} onClick={() => void saveSeasonResetRule.mutateAsync()}>Save Reset Rule</Button>
      </div>
    </Panel>
  );
}
