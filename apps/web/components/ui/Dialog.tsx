import * as React from "react";

import AppModalShell from "./AppModalShell";

/** Named dialog API over the established responsive GeoDuels modal shell. */
export const Dialog = AppModalShell;
export const AlertDialog = (props: React.ComponentProps<typeof AppModalShell>) => <AppModalShell {...props} role="alertdialog" />;
