import { Tabs, type TabItem } from "../../../components/ui/Tabs";
import { DiscreteSlider } from "../../../components/ui/DiscreteSlider";
import { useMapDetails } from "../../maps/lib/map-hooks";
import { mapThumbnailURL } from "../../maps/lib/map-thumbnails";
import type { MatchConfig, GameRuleset } from "../../matchmaking/lib/queue-client";
import type { PartySnapshot, PartyMode } from "../lib/party-client";
import { CLOCK_OPTIONS, PRESSURE_OPTIONS } from "../lib/lobby-ui";
import { LobbyFieldLabel } from "./lobby-primitives";
import { MapPreview } from "./maps/MapPreview";
import { MapCard } from "./maps/MapPanels";

export function PartySettings({
  accessToken,
  userId,
  busy,
  clockOn,
  config,
  isOwner,
  mode,
  pressureOn,
  pressureSeconds,
  roundSeconds,
  saveConfig,
  saveMode,
  setMapPickerOpen,
  snapshot,
}: {
  accessToken: string;
  userId: string;
  busy: boolean;
  clockOn: boolean;
  config: MatchConfig;
  isOwner: boolean;
  mode: PartyMode;
  pressureOn: boolean;
  pressureSeconds: number;
  roundSeconds: number;
  saveConfig: (patch: MatchConfig) => void;
  saveMode: (mode: PartyMode) => void;
  setMapPickerOpen: (open: boolean) => void;
  snapshot: PartySnapshot;
}) {
  const mapQuery = useMapDetails(accessToken, config.mapId || "", userId);
  const map = mapQuery.data?.map;
  const disabled = busy || !isOwner || snapshot.state !== "open";

  return (
    <div className="grid min-w-0 gap-5 sm:grid-cols-2">
      {map ? (
        <MapCard
          item={map}
          mode="edit"
          thumbnailURL={(item) => mapThumbnailURL(item.thumbnailKey, item.thumbnailVariant)}
          onSelect={() => setMapPickerOpen(true)}
          disabled={disabled}
          className="sm:col-span-2"
        />
      ) : (
        <MapPreview
          name={config.mapName || "World"}
          imageURL={mapThumbnailURL()}
          onClick={() => setMapPickerOpen(true)}
          disabled={disabled}
          actionLabel="Change map"
          className="sm:col-span-2"
        />
      )}
      <SettingsChoice
        label="Mode"
        value={mode}
        options={[{ id: "duel", label: "Duel" }, { id: "team_duel", label: "Team Duel" }, { id: "free_for_all", label: "Free For All" }]}
        onChange={saveMode}
        disabled={disabled}
        className="sm:col-span-2"
      />
      <SettingsChoice
        label="Multipliers"
        value={config.multiplierMode || "shared"}
        options={[{ id: "shared", label: "Shared" }, { id: "individual", label: "Individual" }]}
        onChange={(multiplierMode) => saveConfig({ multiplierMode })}
        disabled={disabled}
      />
      <SettingsChoice<GameRuleset>
        label="Rules"
        value={config.ruleset || "moving"}
        options={[{ id: "moving", label: "Moving" }, { id: "nmpz", label: "NMPZ" }]}
        onChange={(ruleset) => saveConfig({ ruleset })}
        disabled={disabled}
      />
      <div className="grid min-w-0 gap-2">
        <LobbyFieldLabel>Clock</LobbyFieldLabel>
        <p className="text-heading-sm font-strong text-content-primary">{clockOn ? `${roundSeconds}s` : "Infinite"}</p>
        <DiscreteSlider
          aria-label="Round clock"
          value={clockOn ? String(roundSeconds) : "infinite"}
          options={CLOCK_OPTIONS}
          onValueChange={(value) => {
            saveConfig(
              value === "infinite"
                ? { roundTimerMode: "none", roundTimeLimitMs: undefined }
                : { roundTimerMode: "fixed", roundTimeLimitMs: Number(value) * 1000 },
            );
          }}
          disabled={disabled}
        />
      </div>
      <div className="grid min-w-0 gap-2">
        <LobbyFieldLabel>Pressure</LobbyFieldLabel>
        <p className="text-heading-sm font-strong text-content-primary">{pressureOn ? `${pressureSeconds}s` : "None"}</p>
        <DiscreteSlider
          aria-label="Guess pressure"
          value={pressureOn ? String(pressureSeconds) : "none"}
          options={PRESSURE_OPTIONS}
          onValueChange={(value) => {
            saveConfig({ pressureTimeLimitMs: value === "none" ? undefined : Number(value) * 1000 });
          }}
          disabled={disabled}
        />
      </div>
    </div>
  );
}

function SettingsChoice<T extends string>({ label, value, options, onChange, disabled, className }: {
  label: string;
  value: T;
  options: TabItem<T>[];
  onChange: (value: T) => void;
  disabled: boolean;
  className?: string;
}) {
  return (
    <div className={className}>
      <div className="mb-2"><LobbyFieldLabel>{label}</LobbyFieldLabel></div>
      <Tabs className="[&>button]:min-w-0 [&>button]:whitespace-normal [&>button]:px-2" appearance="segmented" aria-label={label} value={value} onChange={onChange} items={options.map((option) => ({ ...option, disabled }))} />
    </div>
  );
}
