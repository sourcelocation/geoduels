import type { AppProps } from 'next/app';
import type { NextPage } from 'next';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Head from 'next/head';
import { Montserrat } from 'next/font/google';
import type { ReactElement, ReactNode } from 'react';
import { useState } from 'react';
import { TooltipProvider } from '../components/ui/Tooltip';
import 'leaflet/dist/leaflet.css';
import 'easymde/dist/easymde.min.css';
import '../styles/globals.css';

const montserrat = Montserrat({
  subsets: ['latin'],
  variable: '--font-montserrat'
});

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
      <div className={montserrat.variable}>
        <TooltipProvider>
          <QueryClientProvider client={queryClient}>
            {getLayout(<Component {...pageProps} />)}
          </QueryClientProvider>
        </TooltipProvider>
      </div>
    </>
  );
}
