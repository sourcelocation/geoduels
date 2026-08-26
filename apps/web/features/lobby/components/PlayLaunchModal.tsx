import { Check, Crosshair, Lock, MousePointer2, Move, X } from "lucide-react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button, IconButton } from "../../../components/ui/button";
import { Switch } from "../../../components/ui/Switch";
import type {
  GameRuleset,
  QueueVariant,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";
import type { ExtensionAvailabilityStatus } from "../../browser-extension/hooks/use-extension-availability";
import { PlayModeActionButton } from "./PlayPanel";

type Props =
  | {
      kind: "duel";
      extensionAvailable: boolean;
      extensionStatus: ExtensionAvailabilityStatus;
      queues: QueueVariant[];
      disabled: boolean;
      onQueuesChange: (queues: QueueVariant[]) => void;
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
      error: string;
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
    <div className="rounded-xl border border-border-default bg-surface-grouped p-3 text-center">
      <p className="text-body-sm font-semibold text-content-secondary">
        {outdated
          ? "Update the official GeoDuels browser extension to unlock these options."
          : "Unlock more options by installing the official GeoDuels browser extension. This is required so GeoDuels can modify Google scripts and stay free."}
      </p>
      <div className="mt-2 flex flex-wrap justify-center gap-2">
        <a
          href="https://chromewebstore.google.com/detail/geoduels-enhancer/ecdjkhpicnccgbkdimbbjbnppnkeelmd"
          className="rounded-md bg-surface-fill px-3 py-1.5 text-label font-strong text-content-primary transition-colors hover:bg-surface-raised"
        >
          {outdated ? "Chrome update" : "Chrome setup"}
        </a>
        <a
          href="https://addons.mozilla.org/en-US/firefox/addon/geoduels-enhancer/"
          className="rounded-md bg-surface-fill px-3 py-1.5 text-label font-strong text-content-primary transition-colors hover:bg-surface-raised"
        >
          {outdated ? "Firefox update" : "Firefox setup"}
        </a>
      </div>
    </div>
  );
}

function StreetNamesToggle({
  hidden,
  locked,
  onChange,
}: {
  hidden: boolean;
  locked: boolean;
  onChange: (hidden: boolean) => void;
}) {
  return (
    <div className="flex justify-center">
      <div className="flex min-h-12 w-full items-center justify-between gap-4 rounded-xl border border-border-default bg-surface-grouped px-4 text-left">
        <span className="flex items-center gap-2 text-body-sm font-strong text-content-primary">
          Hide street names
          {locked ? <Lock size={15} className="text-content-secondary" /> : null}
        </span>
        <Switch
          checked={hidden}
          onCheckedChange={onChange}
          aria-label="Hide street names"
        />
      </div>
    </div>
  );
}

export function PlayLaunchModal(props: Props) {
  const selectedModes = props.kind === "singleplayer" ? [props.mode] : [];
  const hasLockedSelection =
    !props.extensionAvailable &&
    (props.kind === "duel"
      ? props.queues.includes("no_move_hidden")
      : selectedModes.includes("no_move") || props.streetNames === "hidden");
  const hideStartButton =
    hasLockedSelection && props.extensionStatus.state === "missing";

  const chooseMode = (mode: GameRuleset) => {
    if (props.kind === "singleplayer") props.onModeChange(mode);
  };

  const chooseStreetNames = (value: StreetNamesVisibility) => {
    if (props.kind === "singleplayer") props.onStreetNamesChange(value);
  };

  const rankedModes: Array<{
    value: QueueVariant;
    title: string;
    detail: string;
    Icon: typeof Move;
    extensionRequired: boolean;
  }> = [
    {
      value: "moving",
      title: "Moving",
      detail: "Street names",
      Icon: Move,
      extensionRequired: false,
    },
    {
      value: "no_move_hidden",
      title: "No Move",
      detail: "No street names",
      Icon: MousePointer2,
      extensionRequired: true,
    },
  ];

  return (
    <AppModalShell
      title={props.kind === "duel" ? "Find a Duel" : "Start Singleplayer"}
      onClose={props.onClose}
      placement="center"
      maxWidthClassName="max-w-xl"
      showHeader={false}
    >
      <IconButton
        onClick={props.onClose}
        aria-label={`Close ${props.kind === "duel" ? "Find a Duel" : "Start Singleplayer"}`}
        className="absolute right-4 top-4 h-8 min-h-8 w-8"
      >
        <X size={18} strokeWidth={2.5} />
      </IconButton>

      <div className="space-y-5 pt-7">
        <div
          className={`grid gap-3 ${props.kind === "duel" ? "sm:grid-cols-2" : "sm:grid-cols-3"}`}
        >
          {(props.kind === "duel" ? rankedModes : modes).map((mode) => {
            if (props.kind === "duel") {
              const selected = props.queues.includes(
                mode.value as QueueVariant,
              );
              const locked =
                mode.extensionRequired && !props.extensionAvailable;
              const Icon = mode.Icon;
              return (
                <Button
                  variant="ghost"
                  type="button"
                  key={mode.value}
                  aria-pressed={selected}
                  aria-label={mode.title}
                  onClick={() =>
                    props.onQueuesChange(
                      selected
                        ? props.queues.filter((queue) => queue !== mode.value)
                        : [...props.queues, mode.value as QueueVariant],
                    )
                  }
                  className={`relative flex min-h-32 flex-col items-center justify-center gap-2 rounded-xl border p-4 text-center transition-colors duration-instant active:scale-[0.99] ${selected ? "border-status-success/75 bg-status-success/15 ring-1 ring-status-success/20" : "border-border-default bg-surface-grouped hover:border-border-strong hover:bg-surface-fill"}`}
                >
                  <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-surface-fill text-content-primary">
                    <Icon size={22} />
                  </span>
                  <span className="text-body font-strong text-content-primary">
                    {mode.title}
                  </span>
                  <span className="text-label font-semibold text-content-secondary">
                    {"detail" in mode ? mode.detail : null}
                  </span>
                  {locked ? (
                    <Lock
                      size={16}
                      className="absolute right-3 top-3 text-content-secondary"
                    />
                  ) : selected ? (
                    <Check
                      size={17}
                      className="absolute right-3 top-3 text-action-primary"
                    />
                  ) : null}
                </Button>
              );
            }
            const selected = selectedModes.includes(mode.value as GameRuleset);
            const locked = mode.extensionRequired && !props.extensionAvailable;
            const Icon = mode.Icon;
            return (
              <Button
                variant="ghost"
                type="button"
                key={mode.value}
                aria-pressed={selected}
                aria-label={mode.title}
                onClick={() => chooseMode(mode.value as GameRuleset)}
                className={`relative flex min-h-32 flex-col items-center justify-center gap-3 rounded-xl border p-4 text-center transition-colors duration-instant active:scale-[0.99] ${
                  selected
                    ? "border-status-success/75 bg-status-success/15 ring-1 ring-status-success/20"
                    : "border-border-default bg-surface-grouped hover:border-border-strong hover:bg-surface-fill"
                }`}
              >
                <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-surface-fill text-content-primary">
                  <Icon size={22} />
                </span>
                <span className="text-body font-strong text-content-primary">
                  {mode.title}
                </span>
                {locked ? (
                  <Lock
                    size={16}
                    className="absolute right-3 top-3 text-content-secondary"
                  />
                ) : selected ? (
                  <Check
                    size={17}
                    className="absolute right-3 top-3 text-action-primary"
                  />
                ) : null}
              </Button>
            );
          })}
        </div>

        {props.kind === "singleplayer" ? (
          <div className="space-y-3">
            <StreetNamesToggle
              hidden={props.streetNames === "hidden"}
              locked={!props.extensionAvailable}
              onChange={(hidden) =>
                chooseStreetNames(hidden ? "hidden" : "shown")
              }
            />
            {hasLockedSelection ? (
              <ExtensionInstallCallout status={props.extensionStatus} />
            ) : null}
          </div>
        ) : hasLockedSelection ? (
          <ExtensionInstallCallout status={props.extensionStatus} />
        ) : null}

        {props.kind === "duel" && props.queues.length === 0 ? (
          <p className="text-body-sm font-semibold text-status-warning">
            Select at least one mode.
          </p>
        ) : null}

        {props.kind === "singleplayer" && props.error ? (
          <p role="alert" className="text-body-sm font-semibold text-status-danger">
            {props.error}
          </p>
        ) : null}

        {hideStartButton ? null : (
          <PlayModeActionButton
            tone={props.kind === "duel" ? "duel" : "singleplayer"}
            onClick={props.onStart}
            disabled={
              props.disabled ||
              hasLockedSelection ||
              (props.kind === "duel" && props.queues.length === 0)
            }
          >
            Start
          </PlayModeActionButton>
        )}
      </div>
    </AppModalShell>
  );
}
