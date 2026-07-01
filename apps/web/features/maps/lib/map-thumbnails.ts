import catalog from "../../../shared/mapthumbnails/catalog.generated.json";

export type MapThumbnailCategory = "generic" | "continents" | "countries";

export type MapThumbnailOption = {
  key: string;
  label: string;
  category: MapThumbnailCategory;
  search: string;
};

export const mapThumbnailOptions = catalog.thumbnails as MapThumbnailOption[];
const mapThumbnailKeys = new Set(mapThumbnailOptions.map((item) => item.key));

export function mapThumbnailURL(key?: string, variant?: number) {
  const fallback = `generic/variant-${Math.max(1, Math.min(5, variant || 1))}`;
  const selected = key && mapThumbnailKeys.has(key) ? key : fallback;
  return `/map-thumbnails/${selected}.webp`;
}

export function validMapThumbnailKey(key: string) {
  return mapThumbnailKeys.has(key);
}
