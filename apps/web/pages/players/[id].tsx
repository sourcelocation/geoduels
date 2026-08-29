import { useRouter } from "next/router";
import { PlayerProfilePage } from "../../features/players/components/PlayerProfilePage";

export default function PlayerRoute() {
  const router = useRouter();
  const playerId = typeof router.query.id === "string" ? router.query.id.trim() : "";
  return <PlayerProfilePage playerId={playerId} />;
}
