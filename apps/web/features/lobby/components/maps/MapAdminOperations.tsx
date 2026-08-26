import type React from "react";
import { Crosshair, MousePointer2, Move, ShieldCheck, Trash2 } from "lucide-react";
import type { CustomMap, GameplayMapRole } from "../../../maps/lib/maps-client";
import { Button } from "../../../../components/ui/button";
import { SectionCard } from "../../../../components/ui/compositions";

export function MapAdminOperations({
  map,
  onDeleteMap,
  onSetOfficial,
  onSetRole,
}: {
  map: CustomMap;
  onDeleteMap: (map: CustomMap) => void;
  onSetOfficial: (mapId: string, official: boolean) => void;
  onSetRole: (mapId: string, role: GameplayMapRole) => void;
}) {
  const ready = map.status === "ready";
  const roles: Array<{
    role: GameplayMapRole;
    label: string;
    active?: boolean;
    icon: React.ReactNode;
  }> = [
    { role: "moving", label: "Moving", active: map.modeMoving, icon: <Move size={16} /> },
    { role: "no_move", label: "No Move", active: map.modeNoMove, icon: <MousePointer2 size={16} /> },
    { role: "nmpz", label: "NMPZ", active: map.modeNmpz, icon: <Crosshair size={16} /> },
  ];

  return (
    <SectionCard className="rounded-2xl p-4 sm:p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h3 className="flex items-center gap-2 text-heading-sm font-strong tracking-heading text-content-primary">
            <ShieldCheck className="text-action-primary" size={19} />
            Admin Map Operations
          </h3>
          <p className="mt-1 text-body-sm font-medium text-content-secondary">
            Promote this ready map or assign it as the site map for a game mode.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={map.official ? "secondary" : "primary"}
            size="sm"
            disabled={!ready}
            onClick={() => onSetOfficial(map.id, !map.official)}
          >
            <ShieldCheck size={15} />
            {map.official ? "Remove Official" : "Mark Official"}
          </Button>
          {roles.map((item) => (
            <Button
              key={item.role}
              type="button"
              variant={item.active ? "secondary" : "ghost"}
              size="sm"
              disabled={!ready || !!item.active}
              onClick={() => onSetRole(map.id, item.role)}
            >
              {item.icon}
              {item.active ? `${item.label} Active` : item.label}
            </Button>
          ))}
          <Button type="button" variant="danger" size="sm" onClick={() => onDeleteMap(map)}>
            <Trash2 size={15} />
            Delete Map
          </Button>
        </div>
      </div>
      {!ready ? <p className="mt-3 text-body-sm font-semibold text-status-warning">Map must be ready before it can be promoted.</p> : null}
    </SectionCard>
  );
}
