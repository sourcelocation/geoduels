import { useEffect, useState } from "react";
import type {
  GameRuleset,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";

export type DuelStreetNamesChoice = StreetNamesVisibility | "any";

type DuelPreferences = {
  modes: GameRuleset[];
  streetNames: DuelStreetNamesChoice;
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
    new Set(value.filter((item): item is GameRuleset => supportedModes.has(item))),
  );
}

function readPreferences() {
  let duel: DuelPreferences = { modes: ["moving"], streetNames: "shown" };
  let singleplayer: SingleplayerPreferences = {
    mode: "moving",
    streetNames: "shown",
  };
  try {
    const storedDuel = JSON.parse(
      window.localStorage.getItem(DUEL_STORAGE_KEY) || "null",
    );
    if (storedDuel && typeof storedDuel === "object") {
      duel = {
        modes: parseModes(storedDuel.modes),
        streetNames:
          storedDuel.streetNames === "hidden" ||
          storedDuel.streetNames === "any"
            ? storedDuel.streetNames
            : "shown",
      };
    } else {
      const legacy = parseModes(
        JSON.parse(window.localStorage.getItem(LEGACY_STORAGE_KEY) || "null"),
      ).filter((mode) => mode !== "no_move");
      if (legacy.length) duel.modes = legacy;
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
  if (!duel.modes.length) duel.modes = ["moving"];
  return { duel, singleplayer };
}

export function usePlayPreferences(extensionAvailable: boolean | null) {
  const [initial] = useState(readPreferences);
  const [duel, setDuel] = useState<DuelPreferences>(initial.duel);
  const [singleplayer, setSingleplayer] =
    useState<SingleplayerPreferences>(initial.singleplayer);

  useEffect(() => {
    if (extensionAvailable !== false) return;
    setDuel((current) => {
      const modes = current.modes.filter((mode) => mode !== "no_move");
      return {
        modes: modes.length ? modes : ["moving"],
        streetNames: "shown",
      };
    });
    setSingleplayer((current) => ({
      mode: current.mode === "no_move" ? "moving" : current.mode,
      streetNames: "shown",
    }));
  }, [extensionAvailable]);

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
