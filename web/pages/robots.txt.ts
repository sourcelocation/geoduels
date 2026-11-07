
import type { GetServerSideProps } from 'next';
import { siteURL } from "../lib/cached-page";

export const getServerSideProps: GetServerSideProps = async ({ res }) => {
  res.setHeader('Content-Type', 'text/plain');
  res.write(`User-agent: *\nAllow: /\n\nSitemap: ${siteURL()}/sitemap.xml\n`);
  res.end();

  return {
    props: {}
  };
};

export default function RobotsTXT() {
  return null;
}
