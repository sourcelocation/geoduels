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
    color: "#dc2626",
    textClassName: "text-[#fca5a5]",
    activeClassName: "border-2 border-red-200 bg-red-600 text-white ring-2 ring-red-400/70 shadow-[0_0_0_2px_rgba(248,113,113,0.35)]",
    inactiveClassName:
      "border border-red-400/40 bg-red-600/25 text-red-100 hover:bg-red-600/40",
  },
  b: {
    id: "b",
    name: "Team Blue",
    fallback: "B",
    color: "#2563eb",
    textClassName: "text-[#93c5fd]",
    activeClassName: "border-2 border-blue-200 bg-blue-600 text-white ring-2 ring-blue-400/70 shadow-[0_0_0_2px_rgba(96,165,250,0.35)]",
    inactiveClassName:
      "border border-blue-400/40 bg-blue-600/25 text-blue-100 hover:bg-blue-600/40",
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
