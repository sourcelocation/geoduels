import NextApp, { type AppContext, type AppProps } from 'next/app';
import type { RuntimeConfig } from '../lib/runtime-config';
import { RuntimeConfigProvider } from '../lib/runtime-config-context';
import type { NextPage } from 'next';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Head from 'next/head';
import type { ReactElement, ReactNode } from 'react';
import { useState } from 'react';
import { TooltipProvider } from '../components/ui/Tooltip';
import { HotkeyProvider } from '../features/hotkeys/components/HotkeyProvider';
import { AuthProvider } from '../features/auth/components/AuthProvider';
import { AppActivityProvider } from '../features/app-shell/components/AppActivityProvider';
import { AppNoticeProvider } from '../components/ui/AppNotice';
import { WarningPrompt } from '../features/notifications/components/WarningPrompt';
import 'leaflet/dist/leaflet.css';
import 'easymde/dist/easymde.min.css';
import '../styles/globals.css';

export type NextPageWithLayout<P = object, IP = P> = NextPage<P, IP> & {
  getLayout?: (page: ReactElement) => ReactNode;
};

type AppPropsWithLayout = AppProps & {
  Component: NextPageWithLayout;
  runtimeConfig: RuntimeConfig;
};

export default function App({ Component, pageProps, runtimeConfig }: AppPropsWithLayout) {
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
  const getLayout = Component.getLayout ?? ((page) => page);

  return (
    <RuntimeConfigProvider config={runtimeConfig}>
      <Head>
        <link rel="icon" href="/icon.v1.png" type="image/png" />
        <link rel="shortcut icon" href="/icon.v1.png" type="image/png" />
        <link rel="apple-touch-icon" href="/icon.v1.png" />
      </Head>
      <div className="font-body">
        <TooltipProvider>
          <QueryClientProvider client={queryClient}>
            <AuthProvider>
              <AppActivityProvider>
                <HotkeyProvider>
                  <AppNoticeProvider>
                    {getLayout(<Component {...pageProps} />)}
                    <WarningPrompt />
                  </AppNoticeProvider>
                </HotkeyProvider>
              </AppActivityProvider>
            </AuthProvider>
          </QueryClientProvider>
        </TooltipProvider>
      </div>
    </RuntimeConfigProvider>
  );
}

// Request-time configuration is serialized by Next with the initial page props.
// This intentionally disables automatic static optimization for these pages.
App.getInitialProps = async (context: AppContext) => {
  const props = await NextApp.getInitialProps(context);
  const runtimeConfig: RuntimeConfig = typeof window === 'undefined'
    ? (await import('../lib/runtime-config.server')).readServerConfig()
    : window.__NEXT_DATA__.props.runtimeConfig;
  if (!runtimeConfig) throw new Error('Missing server-provided application configuration.');
  return { ...props, runtimeConfig };
};
