import { mapThumbnailURL } from "./map-thumbnails";
import type { CustomMap, MapCountryStat } from "./maps-client";
import {
  formatSocialCount,
  SOCIAL_ICON_IMAGE,
  type SocialPreview,
} from "../../../lib/social-preview";
import { toPublicEntityId } from "../../../lib/entity-id";

const GENERIC_MAP: SocialPreview = {
  title: "GeoDuels | Map Details",
  description:
    "View GeoDuels map details, country distribution, comments, and play actions.",
  canonicalPath: "/maps",
  robots: "index,follow",
  ...SOCIAL_ICON_IMAGE,
};

export function buildMapSocialPreview(
  map: CustomMap | null,
  countryStats: MapCountryStat[] | null,
  routeId: string,
): SocialPreview {
  const canonicalPath = map
    ? `/maps/${encodeURIComponent(toPublicEntityId(map.id))}`
    : routeId
      ? `/maps/${encodeURIComponent(routeId)}`
      : "/maps";
  if (!map) {
    // Missing, or private to someone signed in: anyone without access sees it isn't there.
    return { ...GENERIC_MAP, canonicalPath, robots: "noindex" };
  }

  const kind = map.official || map.system
    ? "Official"
    : map.authorName
      ? `by ${map.authorName}`
      : "Community";
  const parts = [
    kind,
    capitalize(map.difficulty),
    `${formatSocialCount(map.locationCount)} locations`,
    `${formatSocialCount(map.playCount)} plays`,
    `${formatSocialCount(map.favoriteCount)} favorites`,
  ];
  const countries = formatCountryStats(countryStats || []);
  if (countries) parts.push(countries);

  return {
    title: `${map.displayName} | GeoDuels`,
    description: parts.join(" · "),
    canonicalPath,
    robots: "index,follow",
    imagePath: mapThumbnailURL(map.thumbnailKey, map.thumbnailVariant),
    imageWidth: 1280,
    imageHeight: 720,
    imageType: "image/webp",
    twitterCard: "summary_large_image",
  };
}

function formatCountryStats(stats: MapCountryStat[]) {
  const named = stats.filter((item) => {
    const country = item.country.trim();
    return country && !/^unknown$/i.test(country);
  });
  const total = named.reduce((sum, item) => sum + item.locationCount, 0);
  if (!total) return "";
  return named
    .slice()
    .sort((a, b) => b.locationCount - a.locationCount)
    .slice(0, 3)
    .map((item) => {
      const percent = Math.round((item.locationCount / total) * 100);
      return `${item.country.trim()} ${percent}%`;
    })
    .join(" · ");
}

function capitalize(value: string) {
  return value ? value.slice(0, 1).toUpperCase() + value.slice(1) : value;
}
