import type React from "react";
import { Play, ShieldCheck, Trash2, Trophy } from "lucide-react";
import type { CustomMap, GameplayMapRole } from "../../../maps/lib/maps-client";
import { LobbyActionButton, LobbyPanel } from "../lobby-primitives";

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
    { role: "ranked_moving", label: "Ranked Moving", active: map.rankedMoving, icon: <Trophy size={16} /> },
    { role: "ranked_nmpz", label: "Ranked NMPZ", active: map.rankedNmpz, icon: <Trophy size={16} /> },
    { role: "singleplayer_moving", label: "Default Moving", active: map.defaultMoving, icon: <Play size={16} fill="currentColor" /> },
    { role: "singleplayer_nmpz", label: "Default NMPZ", active: map.defaultNmpz, icon: <Play size={16} fill="currentColor" /> },
  ];

  return (
    <LobbyPanel variant="subtle" className="p-4 sm:p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h3 className="flex items-center gap-2 text-lg font-extrabold tracking-tight text-white">
            <ShieldCheck className="text-accentPrimary" size={19} />
            Admin Map Operations
          </h3>
          <p className="mt-1 text-sm font-medium text-inkMuted">
            Promote this ready map or assign it to ranked and default queues.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <LobbyActionButton
            type="button"
            variant={map.official ? "secondary" : "primary"}
            size="sm"
            disabled={!ready}
            onClick={() => onSetOfficial(map.id, !map.official)}
          >
            <ShieldCheck size={15} />
            {map.official ? "Remove Official" : "Mark Official"}
          </LobbyActionButton>
          {roles.map((item) => (
            <LobbyActionButton
              key={item.role}
              type="button"
              variant={item.active ? "secondary" : "ghost"}
              size="sm"
              disabled={!ready || !!item.active}
              onClick={() => onSetRole(map.id, item.role)}
            >
              {item.icon}
              {item.active ? `${item.label} Active` : item.label}
            </LobbyActionButton>
          ))}
          <LobbyActionButton type="button" variant="danger" size="sm" onClick={() => onDeleteMap(map)}>
            <Trash2 size={15} />
            Delete Map
          </LobbyActionButton>
        </div>
      </div>
      {!ready ? <p className="mt-3 text-xs font-semibold text-amber-200">Map must be ready before it can be promoted.</p> : null}
    </LobbyPanel>
  );
}
