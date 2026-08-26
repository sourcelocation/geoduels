import Document, { Head, Html, Main, NextScript } from 'next/document';
import { outfit } from '../styles/fonts';

export default class MyDocument extends Document {
  render() {
    return (
      <Html lang="en" className={outfit.variable}>
        <Head>
          <meta
            name="viewport"
            content="width=device-width, initial-scale=1, viewport-fit=cover"
          />
        </Head>
        <body>
          <script src="/runtime-config.js" />
          <Main />
          <NextScript />
        </body>
      </Html>
    );
  }
}
