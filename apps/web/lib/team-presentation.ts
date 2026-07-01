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
    activeClassName: "bg-[#dc2626] text-white",
    inactiveClassName:
      "border border-[#f87171]/25 bg-[#dc2626]/15 text-[#fecaca] hover:bg-[#dc2626]/25",
  },
  b: {
    id: "b",
    name: "Team Blue",
    fallback: "B",
    color: "#2563eb",
    textClassName: "text-[#93c5fd]",
    activeClassName: "bg-[#2563eb] text-white",
    inactiveClassName:
      "border border-[#60a5fa]/25 bg-[#2563eb]/15 text-[#bfdbfe] hover:bg-[#2563eb]/25",
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
