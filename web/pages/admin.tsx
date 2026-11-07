import AdminDashboard from "../features/admin/AdminDashboard";
import type { NextPageWithLayout } from "./_app";

const AdminPage: NextPageWithLayout = function AdminPage() {
  return <AdminDashboard />;
};

AdminPage.surface = "admin";

export default AdminPage;
