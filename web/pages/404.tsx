import Head from "next/head";
import { AppContentRail } from "../features/app-shell/components/AppContentRail";
import { ButtonLink } from "../components/ui/button";
import { AppPanel } from "../components/ui/compositions";

export default function NotFoundPage() {
  return (
    <>
      <Head>
        <title>Page not found | GeoDuels</title>
        <meta name="robots" content="noindex" />
      </Head>
      <AppContentRail>
        <AppPanel className="rounded-2xl p-10 text-center">
          <h1 className="text-heading-lg font-strong text-content-primary">Page not found</h1>
          <p className="mt-3 text-content-secondary">This page does not exist or is no longer available.</p>
          <ButtonLink href="/" variant="primary" className="mt-6">Back to GeoDuels</ButtonLink>
        </AppPanel>
      </AppContentRail>
    </>
  );
}
