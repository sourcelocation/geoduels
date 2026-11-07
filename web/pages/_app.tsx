import type { AppProps } from 'next/app';
import type { NextPage } from 'next';
import { HydrationBoundary, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Head from 'next/head';
import type { ReactElement, ReactNode } from 'react';
import { useState } from 'react';
import { SiteURLProvider } from '../lib/site';
import { TooltipProvider } from '../components/ui/Tooltip';
import { HotkeyProvider } from '../features/hotkeys/components/HotkeyProvider';
import { AuthProvider } from '../features/auth/components/AuthProvider';
import { AppActivityProvider } from '../features/app-shell/components/AppActivityProvider';
import { AppNoticeProvider } from '../components/ui/AppNotice';
import { Notifications } from '../features/notifications/components/Notifications';
import { AppShell } from '../features/app-shell/components/AppShell';
import { DirectChat } from '../features/chat/components/DirectChat';
import { LiveProvider } from '../features/live/LiveProvider';
import 'leaflet/dist/leaflet.css';
import 'easymde/dist/easymde.min.css';
import '../styles/globals.css';

export type NextPageWithLayout<P = object, IP = P> = NextPage<P, IP> & {
  getLayout?: (page: ReactElement) => ReactNode;
  // The staff tools are the one thing that isn't inside GeoDuels' frame.
  surface?: "admin";
};

type AppPropsWithLayout = AppProps & {
  Component: NextPageWithLayout;
};

export default function App({ Component, pageProps }: AppPropsWithLayout) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            refetchOnWindowFocus: false,
            retry: false
          }
        }
      })
  );
  const getLayout = Component.getLayout ?? ((content) => content);
  const page = getLayout(<Component {...pageProps} />);

  return (
    <SiteURLProvider value={pageProps.siteURL ?? ''}>
      <Head>
        <link rel="icon" href="/icon.v1.png" type="image/png" />
        <link rel="shortcut icon" href="/icon.v1.png" type="image/png" />
        <link rel="apple-touch-icon" href="/icon.v1.png" />
      </Head>
      <div className="font-body">
        <TooltipProvider>
          <QueryClientProvider client={queryClient}>
            <HydrationBoundary state={pageProps.dehydratedState}>
            <AuthProvider>
              <AppActivityProvider>
                <HotkeyProvider>
                  <AppNoticeProvider>
                    <LiveProvider>
                      {Component.surface === 'admin' ? page : <AppShell>{page}</AppShell>}
                      <Notifications />
                      <DirectChat />
                    </LiveProvider>
                  </AppNoticeProvider>
                </HotkeyProvider>
              </AppActivityProvider>
            </AuthProvider>
            </HydrationBoundary>
          </QueryClientProvider>
        </TooltipProvider>
      </div>
    </SiteURLProvider>
  );
}

