import { useEffect, useState } from "react";
import type {
  GameRuleset,
  QueueVariant,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";

type DuelPreferences = {
  queues: QueueVariant[];
};

type SingleplayerPreferences = {
  mode: GameRuleset;
  streetNames: StreetNamesVisibility;
};

const DUEL_STORAGE_KEY = "geoduels.play.duels";
const SINGLEPLAYER_STORAGE_KEY = "geoduels.play.singleplayer";
const LEGACY_STORAGE_KEY = "geoduels.queueRulesets";

const supportedModes = new Set<GameRuleset>(["moving", "no_move", "nmpz"]);

function parseModes(value: unknown): GameRuleset[] {
  if (!Array.isArray(value)) return [];
  return Array.from(
    new Set(
      value.filter((item): item is GameRuleset => supportedModes.has(item)),
    ),
  );
}

const rankedQueues = new Set<QueueVariant>(["moving", "no_move_hidden"]);

function parseRankedQueues(value: unknown): QueueVariant[] {
  if (!Array.isArray(value)) return [];
  return Array.from(
    new Set(
      value.filter((item): item is QueueVariant => rankedQueues.has(item)),
    ),
  );
}

function readPreferences() {
  let duel: DuelPreferences = { queues: ["moving"] };
  let singleplayer: SingleplayerPreferences = {
    mode: "moving",
    streetNames: "shown",
  };
  try {
    const storedDuel = JSON.parse(
      window.localStorage.getItem(DUEL_STORAGE_KEY) || "null",
    );
    if (storedDuel && typeof storedDuel === "object") {
      const queues = parseRankedQueues(storedDuel.queues);
      if (queues.length) {
        duel = { queues };
      } else {
        const modes = parseModes(storedDuel.modes);
        const streetNames = storedDuel.streetNames;
        const migrated: QueueVariant[] = [];
        if (modes.includes("moving") && streetNames !== "hidden")
          migrated.push("moving");
        if (modes.includes("no_move") && streetNames !== "shown")
          migrated.push("no_move_hidden");
        if (migrated.length) duel = { queues: migrated };
      }
    } else {
      const legacy = parseModes(
        JSON.parse(window.localStorage.getItem(LEGACY_STORAGE_KEY) || "null"),
      );
      if (legacy.includes("moving")) duel.queues = ["moving"];
    }

    const storedSingleplayer = JSON.parse(
      window.localStorage.getItem(SINGLEPLAYER_STORAGE_KEY) || "null",
    );
    if (
      storedSingleplayer &&
      typeof storedSingleplayer === "object" &&
      supportedModes.has(storedSingleplayer.mode)
    ) {
      singleplayer = {
        mode: storedSingleplayer.mode,
        streetNames:
          storedSingleplayer.streetNames === "hidden" ? "hidden" : "shown",
      };
    }
  } catch {
    // Defaults keep the launcher usable when storage is unavailable or invalid.
  }
  if (!duel.queues.length) duel.queues = ["moving"];
  return { duel, singleplayer };
}

export function usePlayPreferences() {
  const [initial] = useState(readPreferences);
  const [duel, setDuel] = useState<DuelPreferences>(initial.duel);
  const [singleplayer, setSingleplayer] = useState<SingleplayerPreferences>(
    initial.singleplayer,
  );

  useEffect(() => {
    try {
      window.localStorage.setItem(DUEL_STORAGE_KEY, JSON.stringify(duel));
      window.localStorage.removeItem(LEGACY_STORAGE_KEY);
    } catch {
      // Storage failures do not block launching.
    }
  }, [duel]);

  useEffect(() => {
    try {
      window.localStorage.setItem(
        SINGLEPLAYER_STORAGE_KEY,
        JSON.stringify(singleplayer),
      );
    } catch {
      // Storage failures do not block launching.
    }
  }, [singleplayer]);

  return { duel, setDuel, singleplayer, setSingleplayer };
}
