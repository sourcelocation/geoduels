import type { AppProps } from 'next/app';
import type { NextPage } from 'next';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Head from 'next/head';
import type { ReactElement, ReactNode } from 'react';
import { useState } from 'react';
import { TooltipProvider } from '../components/ui/Tooltip';
import { HotkeyProvider } from '../features/hotkeys/components/HotkeyProvider';
import { SocialRealtimeProvider } from '../features/social/components/SocialRealtimeProvider';
import { AuthProvider } from '../features/auth/components/AuthProvider';
import { AppActivityProvider } from '../features/app-shell/components/AppActivityProvider';
import 'leaflet/dist/leaflet.css';
import 'easymde/dist/easymde.min.css';
import '../styles/globals.css';

export type NextPageWithLayout = NextPage & {
  getLayout?: (page: ReactElement) => ReactNode;
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
  const getLayout = Component.getLayout ?? ((page) => page);

  return (
    <>
      <Head>
        <link rel="icon" href="/icon.v1.png" type="image/png" />
        <link rel="shortcut icon" href="/icon.v1.png" type="image/png" />
        <link rel="apple-touch-icon" href="/icon.v1.png" />
      </Head>
      <div className="font-body">
        <TooltipProvider>
          <QueryClientProvider client={queryClient}>
            <AuthProvider>
              <SocialRealtimeProvider>
                <AppActivityProvider>
                  <HotkeyProvider>
                    {getLayout(<Component {...pageProps} />)}
                  </HotkeyProvider>
                </AppActivityProvider>
              </SocialRealtimeProvider>
            </AuthProvider>
          </QueryClientProvider>
        </TooltipProvider>
      </div>
    </>
  );
}
