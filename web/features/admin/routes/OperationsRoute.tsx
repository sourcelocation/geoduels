import { Heading, Text } from "../../../components/ui/typography";
import { AdminPanel as Panel } from "../components/admin-primitives";
import { ChangelogSection } from "./operations/ChangelogSection";
import { CommunityPardonSection } from "./operations/CommunityPardonSection";
import { DiscordSection } from "./operations/DiscordSection";
import { IpSignupBlocksSection } from "./operations/IpSignupBlocksSection";
import { MaintenanceSection } from "./operations/MaintenanceSection";
import { NotificationsSection } from "./operations/NotificationsSection";
import { SeasonsSection } from "./operations/SeasonsSection";

export function OperationsRoute(props: {
  accessToken: string;
  leaf: string;
  canManageAdmin: boolean;
  refreshAdminData: () => Promise<void>;
}) {
  if (!props.canManageAdmin) {
    return <Panel className="p-5 text-body-sm text-content-secondary">Admin access is required for operations.</Panel>;
  }

  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">Operations</Text>
        <Heading as="h2" variant="display-md" className="mt-1">Admin Operations</Heading>
      </header>
      <CommunityPardonSection {...props} />
      <div className="grid gap-4 xl:grid-cols-2">
        <MaintenanceSection {...props} />
        <NotificationsSection {...props} />
        <DiscordSection {...props} />
        <SeasonsSection {...props} />
        <ChangelogSection {...props} />
        <IpSignupBlocksSection {...props} />
      </div>
    </div>
  );
}
