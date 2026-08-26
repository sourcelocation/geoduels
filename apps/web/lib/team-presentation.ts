export type TeamPresentation = {
  id: string;
  name: string;
  fallback: string;
  color: string;
  textClassName: string;
  activeClassName: string;
  inactiveClassName: string;
};

const TEAM_PRESENTATIONS: Record<string, TeamPresentation> = {
  a: {
    id: "a",
    name: "Team Red",
    fallback: "R",
    color: "rgb(var(--gd-status-danger))",
    textClassName: "text-status-danger",
    activeClassName: "bg-status-danger text-content-on-danger",
    inactiveClassName:
      "border border-status-danger/25 bg-status-danger/15 text-status-danger hover:bg-status-danger/25",
  },
  b: {
    id: "b",
    name: "Team Blue",
    fallback: "B",
    color: "rgb(var(--gd-status-info))",
    textClassName: "text-status-info",
    activeClassName: "bg-status-info text-content-on-action",
    inactiveClassName:
      "border border-status-info/25 bg-status-info/15 text-status-info hover:bg-status-info/25",
  },
};

export function getTeamPresentation(
  teamId?: string,
  name?: string,
): TeamPresentation {
  const base = TEAM_PRESENTATIONS[teamId || "a"] || {
    ...TEAM_PRESENTATIONS.a,
    id: teamId || "team",
    name: "Team",
    fallback: "T",
  };
  return name ? { ...base, name } : base;
}
