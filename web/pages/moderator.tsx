import AdminDashboard from "../features/admin/AdminDashboard";
import type { NextPageWithLayout } from "./_app";

const ModeratorPage: NextPageWithLayout = function ModeratorPage() {
  return <AdminDashboard surface="moderator" />;
};

ModeratorPage.surface = "admin";

export default ModeratorPage;
