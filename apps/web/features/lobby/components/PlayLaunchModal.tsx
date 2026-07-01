import { Check, Crosshair, Lock, MousePointer2, Move, X } from "lucide-react";
import AppModalShell from "../../../components/ui/AppModalShell";
import type {
  GameRuleset,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";
import type { ExtensionAvailabilityStatus } from "../../browser-extension/hooks/use-extension-availability";
import type { DuelStreetNamesChoice } from "../hooks/usePlayPreferences";
import { PlayModeActionButton } from "./PlayPanel";

type Props =
  | {
      kind: "duel";
      extensionAvailable: boolean;
      extensionStatus: ExtensionAvailabilityStatus;
      modes: GameRuleset[];
      streetNames: DuelStreetNamesChoice;
      disabled: boolean;
      onModesChange: (modes: GameRuleset[]) => void;
      onStreetNamesChange: (value: DuelStreetNamesChoice) => void;
      onClose: () => void;
      onStart: () => void;
    }
  | {
      kind: "singleplayer";
      extensionAvailable: boolean;
      extensionStatus: ExtensionAvailabilityStatus;
      mode: GameRuleset;
      streetNames: StreetNamesVisibility;
      disabled: boolean;
      onModeChange: (mode: GameRuleset) => void;
      onStreetNamesChange: (value: StreetNamesVisibility) => void;
      onClose: () => void;
      onStart: () => void;
    };

const modes: Array<{
  value: GameRuleset;
  title: string;
  Icon: typeof Move;
  extensionRequired: boolean;
}> = [
  {
    value: "moving",
    title: "Moving",
    Icon: Move,
    extensionRequired: false,
  },
  {
    value: "no_move",
    title: "No Move",
    Icon: MousePointer2,
    extensionRequired: true,
  },
  {
    value: "nmpz",
    title: "NMPZ",
    Icon: Crosshair,
    extensionRequired: false,
  },
];

function ExtensionInstallCallout({
  status,
}: {
  status: ExtensionAvailabilityStatus;
}) {
  const outdated = status.state === "outdated";
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.04] p-3 text-center">
      <p className="text-xs font-semibold leading-5 text-white/70">
        {outdated
          ? "Update the official GeoDuels browser extension to unlock these options."
          : "Unlock more options by installing the official GeoDuels browser extension."}
      </p>
      <div className="mt-2 flex flex-wrap justify-center gap-2">
        <a
          href="https://chromewebstore.google.com/detail/geoduels-enhancer/ecdjkhpicnccgbkdimbbjbnppnkeelmd"
          className="rounded-lg bg-white/10 px-3 py-1.5 text-xs font-black text-white transition-colors hover:bg-white/15"
        >
          {outdated ? "Chrome update" : "Chrome setup"}
        </a>
        <a
          href="https://addons.mozilla.org/en-US/firefox/addon/geoduels-enhancer/"
          className="rounded-lg bg-white/10 px-3 py-1.5 text-xs font-black text-white transition-colors hover:bg-white/15"
        >
          {outdated ? "Firefox update" : "Firefox setup"}
        </a>
      </div>
    </div>
  );
}

function DisabledStreetNamesToggle() {
  return (
    <div className="flex justify-center">
      <button
        type="button"
        aria-disabled="true"
        aria-label="Hide street names requires the GeoDuels extension"
        className="flex min-h-12 w-full cursor-not-allowed items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/[0.04] px-4 text-left opacity-65"
      >
        <span className="text-sm font-black text-white">Hide street names</span>
        <span className="relative h-6 w-11 rounded-full bg-white/10 shadow-inner">
          <span className="absolute left-1 top-1 h-4 w-4 rounded-full bg-white/45" />
        </span>
      </button>
    </div>
  );
}

export function PlayLaunchModal(props: Props) {
  const selectedModes =
    props.kind === "duel" ? props.modes : [props.mode];
  const hasLockedSelection =
    !props.extensionAvailable &&
    (selectedModes.includes("no_move") || props.streetNames !== "shown");
  const streetOptions: Array<{
    value: DuelStreetNamesChoice;
    label: string;
    extensionRequired: boolean;
  }> = [
    { value: "shown", label: "Shown", extensionRequired: false },
    { value: "hidden", label: "Hidden", extensionRequired: true },
    ...(props.kind === "duel"
      ? [{ value: "any" as const, label: "Any", extensionRequired: true }]
      : []),
  ];

  const chooseMode = (mode: GameRuleset) => {
    if (mode === "no_move" && !props.extensionAvailable) return;
    if (props.kind === "singleplayer") {
      props.onModeChange(mode);
      return;
    }
    props.onModesChange(
      props.modes.includes(mode)
        ? props.modes.filter((current) => current !== mode)
        : [...props.modes, mode],
    );
  };

  const chooseStreetNames = (value: DuelStreetNamesChoice) => {
    if (value !== "shown" && !props.extensionAvailable) return;
    if (props.kind === "duel") {
      props.onStreetNamesChange(value);
    } else if (value !== "any") {
      props.onStreetNamesChange(value);
    }
  };

  return (
    <AppModalShell
      title={props.kind === "duel" ? "Find a Duel" : "Start Singleplayer"}
      onClose={props.onClose}
      placement="center"
      maxWidthClassName="max-w-xl"
      showHeader={false}
      panelClassName="relative"
    >
      <button
        type="button"
        onClick={props.onClose}
        aria-label={`Close ${props.kind === "duel" ? "Find a Duel" : "Start Singleplayer"}`}
        className="absolute right-4 top-4 flex h-8 w-8 items-center justify-center rounded-full bg-white/5 text-white/70 transition hover:bg-white/10 hover:text-white"
      >
        <X size={18} strokeWidth={2.5} />
      </button>

      <div className="space-y-5 pt-7">
        <div className="grid gap-3 sm:grid-cols-3">
          {modes.map((mode) => {
            const selected = selectedModes.includes(mode.value);
            const locked = mode.extensionRequired && !props.extensionAvailable;
            const Icon = mode.Icon;
            return (
              <button
                type="button"
                key={mode.value}
                aria-pressed={selected}
                aria-label={mode.title}
                disabled={locked}
                onClick={() => chooseMode(mode.value)}
                className={`relative flex min-h-32 flex-col items-center justify-center gap-3 rounded-2xl border p-4 text-center transition-colors duration-75 active:scale-[0.99] ${
                  selected
                    ? "border-accentPrimary/75 bg-accentPrimary/15 ring-1 ring-accentPrimary/20"
                    : "border-white/10 bg-white/[0.045] hover:border-white/20 hover:bg-white/[0.075]"
                } ${locked ? "cursor-not-allowed opacity-45" : ""}`}
              >
                <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/[0.07] text-white">
                  <Icon size={22} />
                </span>
                <span className="text-base font-black text-white">
                  {mode.title}
                </span>
                {locked ? (
                  <Lock size={16} className="absolute right-3 top-3 text-white/55" />
                ) : selected ? (
                  <Check size={17} className="absolute right-3 top-3 text-accentPrimary" />
                ) : null}
              </button>
            );
          })}
        </div>

        <div className="space-y-3">
          {props.extensionAvailable ? (
            <div className="grid rounded-xl border border-white/10 bg-black/20 p-1" style={{ gridTemplateColumns: `repeat(${streetOptions.length}, minmax(0, 1fr))` }}>
              {streetOptions.map((option) => {
                const selected = props.streetNames === option.value;
                return (
                  <button
                    key={option.value}
                    type="button"
                    aria-pressed={selected}
                    onClick={() => chooseStreetNames(option.value)}
                    className={`flex min-h-11 items-center justify-center rounded-lg px-3 text-[11px] font-black uppercase tracking-[0.08em] transition ${
                      selected
                        ? "bg-white text-[#10201a]"
                        : "text-[#a9bfd4] hover:bg-white/[0.08]"
                    }`}
                  >
                    {option.label}
                  </button>
                );
              })}
            </div>
          ) : (
            <>
              <DisabledStreetNamesToggle />
              <ExtensionInstallCallout status={props.extensionStatus} />
            </>
          )}
        </div>

        {props.kind === "duel" && props.modes.length === 0 ? (
          <p className="text-sm font-semibold text-amber-200">
            Select at least one mode.
          </p>
        ) : null}

        <PlayModeActionButton
          tone={props.kind === "duel" ? "duel" : "singleplayer"}
          onClick={props.onStart}
          disabled={
            props.disabled ||
            hasLockedSelection ||
            (props.kind === "duel" && props.modes.length === 0)
          }
        >
          Start
        </PlayModeActionButton>
      </div>
    </AppModalShell>
  );
}
