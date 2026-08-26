import { ExternalLink, Loader2, Users, X } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ButtonLink, IconButton } from "../../../components/ui/button";
import { AppNavigationSurface } from "../../../components/ui/compositions";

export type AppNavTask =
  | {
      kind: "queue";
      label: string;
      onCancel: () => void;
    }
  | {
      kind: "party";
      label: string;
      href: string;
    };

export function AppNavTasks({ tasks }: { tasks: AppNavTask[] }) {
  const reduceMotion = useReducedMotion();

  return (
    <AnimatePresence initial={false} mode="popLayout">
      {tasks.length ? (
        <motion.div
          key="tasks"
          layout
          initial={reduceMotion ? false : { opacity: 0, scale: 0.92, x: 12 }}
          animate={{ opacity: 1, scale: 1, x: 0 }}
          exit={reduceMotion ? undefined : { opacity: 0, scale: 0.92, x: 12 }}
          transition={{ type: "spring", stiffness: 420, damping: 34 }}
          className="min-w-0 shrink-0"
        >
          <AppNavigationSurface
            as="div"
            aria-label="Active tasks"
            className="pointer-events-auto flex min-h-16 min-w-0 flex-col items-stretch gap-1 overflow-hidden rounded-xl p-1.5 sm:flex-row sm:gap-0 sm:divide-x sm:divide-border-default md:min-h-12 md:rounded-full"
          >
            {tasks.map((task) => (
              <div key={task.kind} className="flex min-w-0 items-center gap-2 px-2 md:px-3">
                {task.kind === "queue" ? (
                  <Loader2 className="shrink-0 animate-spin text-status-success" size={18} aria-hidden="true" />
                ) : (
                  <Users className="shrink-0 text-status-success" size={18} aria-hidden="true" />
                )}
                <span className="max-w-20 truncate whitespace-nowrap text-label font-strong text-content-primary sm:max-w-44 md:text-body-sm">
                  {task.label}
                </span>
                {task.kind === "queue" ? (
                  <IconButton
                    aria-label="Cancel matchmaking"
                    title="Cancel matchmaking"
                    onClick={task.onCancel}
                    className="h-9 min-h-9 w-9 shrink-0"
                  >
                    <X size={16} aria-hidden="true" />
                  </IconButton>
                ) : (
                  <ButtonLink
                    aria-label="Return to party lobby"
                    title="Return to party lobby"
                    href={task.href}
                    variant="icon"
                    size="icon"
                    className="h-9 min-h-9 w-9 shrink-0"
                  >
                    <ExternalLink size={16} aria-hidden="true" />
                  </ButtonLink>
                )}
              </div>
            ))}
          </AppNavigationSurface>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
