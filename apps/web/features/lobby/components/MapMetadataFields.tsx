import { Pencil } from "lucide-react";
import { useState } from "react";
import type { CustomMap, MapVisibility } from "../../maps/lib/maps-client";
import { mapThumbnailOptions, mapThumbnailURL } from "../../maps/lib/map-thumbnails";
import { MapThumbnailPickerModal, type ThumbnailCategory } from "./MapThumbnailPickerModal";
import {
  LobbyInput,
  LobbySegmentedControl,
  LobbyTextarea,
} from "./lobby-primitives";

type MapDifficulty = CustomMap["difficulty"];

type MapMetadataFieldsProps = {
  disabled?: boolean;
  mapName: string;
  setMapName: (value: string) => void;
  mapDescription: string;
  setMapDescription: (value: string) => void;
  mapDifficulty: MapDifficulty;
  setMapDifficulty: (value: MapDifficulty) => void;
  mapVisibility: MapVisibility;
  setMapVisibility: (value: MapVisibility) => void;
  mapThumbnailKey: string;
  setMapThumbnailKey: (value: string) => void;
  mapThumbnailCategory: ThumbnailCategory;
  setMapThumbnailCategory: (value: ThumbnailCategory) => void;
  mapThumbnailSearch: string;
  setMapThumbnailSearch: (value: string) => void;
  autoZoomPlayRegion?: boolean;
  setAutoZoomPlayRegion?: (value: boolean) => void;
};

export function thumbnailCategoryFromKey(key: string): ThumbnailCategory {
  const category = key.split("/")[0];
  return category === "continents" || category === "countries" ? category : "generic";
}

export function MapMetadataFields({
  disabled = false,
  mapName,
  setMapName,
  mapDescription,
  setMapDescription,
  mapDifficulty,
  setMapDifficulty,
  mapVisibility,
  setMapVisibility,
  mapThumbnailKey,
  setMapThumbnailKey,
  mapThumbnailCategory,
  setMapThumbnailCategory,
  mapThumbnailSearch,
  setMapThumbnailSearch,
  autoZoomPlayRegion,
  setAutoZoomPlayRegion,
}: MapMetadataFieldsProps) {
  const [thumbnailPickerOpen, setThumbnailPickerOpen] = useState(false);
  const selectedThumbnail =
    mapThumbnailOptions.find((item) => item.key === mapThumbnailKey) ||
    mapThumbnailOptions[0];
  const difficultyOptions: Array<{ value: MapDifficulty; label: string }> = [
    { value: "easy", label: "Easy" },
    { value: "normal", label: "Normal" },
    { value: "hard", label: "Hard" },
  ];
  const visibilityOptions: Array<{ value: MapVisibility; label: string }> = [
    { value: "private", label: "Private" },
    { value: "unlisted", label: "Unlisted" },
    { value: "public", label: "Public" },
  ];

  return (
    <>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
        <div className="grid gap-4">
          <LobbyInput value={mapName} onChange={(event) => setMapName(event.target.value)} maxLength={80} placeholder="Map name" disabled={disabled} className="h-11 rounded-xl font-semibold" aria-label="Map name" />
          <LobbyTextarea value={mapDescription} onChange={(event) => setMapDescription(event.target.value)} maxLength={500} placeholder="Description (optional)" disabled={disabled} className="min-h-24 resize-none rounded-xl" aria-label="Description" />

          <div className="grid gap-2">
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">Difficulty</p>
            <LobbySegmentedControl
              value={mapDifficulty}
              items={difficultyOptions}
              onChange={(value) => {
                if (!disabled) setMapDifficulty(value);
              }}
              className={disabled ? "pointer-events-none opacity-50" : undefined}
            />
          </div>

          <div className="grid gap-2">
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">Visibility</p>
            <LobbySegmentedControl
              value={mapVisibility}
              items={visibilityOptions}
              onChange={(value) => {
                if (!disabled) setMapVisibility(value);
              }}
              className={disabled ? "pointer-events-none opacity-50" : undefined}
            />
          </div>

          {setAutoZoomPlayRegion ? (
            <label className={`flex items-start gap-3 rounded-xl border border-white/10 bg-black/20 p-3 ${disabled ? "pointer-events-none opacity-50" : "cursor-pointer"}`}>
              <input
                type="checkbox"
                checked={!!autoZoomPlayRegion}
                disabled={disabled}
                onChange={(event) => setAutoZoomPlayRegion(event.target.checked)}
                className="mt-0.5 h-4 w-4 shrink-0 accent-[#2ad18f]"
              />
              <span className="grid gap-1">
                <span className="text-sm font-bold text-white">Auto-zoom minimap to play region</span>
                <span className="text-xs font-semibold text-[#6b8b80]">Zooms the guess minimap to this map&apos;s location bounds when a round starts.</span>
              </span>
            </label>
          ) : null}
        </div>

        <div className="grid content-start gap-2">
          <p className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">Thumbnail</p>
          <button
            type="button"
            disabled={disabled}
            onClick={() => setThumbnailPickerOpen(true)}
            className="group relative overflow-hidden rounded-xl text-left transition disabled:cursor-not-allowed disabled:opacity-50"
            aria-label={`Choose thumbnail. Current thumbnail: ${selectedThumbnail.label}`}
          >
            <img src={mapThumbnailURL(mapThumbnailKey)} alt="" className="aspect-[16/9] w-full object-cover" />
            <span className="absolute bottom-3 left-3 rounded-full bg-black/70 px-3 py-1 text-xs font-extrabold text-white">
              {selectedThumbnail.label}
            </span>
            <span className="absolute right-3 top-3 inline-flex h-10 w-10 items-center justify-center rounded-full bg-black/70 text-white transition group-hover:bg-accentPrimary">
              <Pencil size={17} aria-hidden="true" />
            </span>
          </button>
        </div>
      </div>
      {thumbnailPickerOpen ? (
        <MapThumbnailPickerModal
          mapThumbnailCategory={mapThumbnailCategory}
          mapThumbnailKey={mapThumbnailKey}
          mapThumbnailSearch={mapThumbnailSearch}
          onClose={() => setThumbnailPickerOpen(false)}
          setMapThumbnailCategory={setMapThumbnailCategory}
          setMapThumbnailKey={setMapThumbnailKey}
          setMapThumbnailSearch={setMapThumbnailSearch}
        />
      ) : null}
    </>
  );
}
