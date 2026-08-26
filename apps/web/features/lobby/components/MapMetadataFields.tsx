import { Pencil } from "lucide-react";
import { useState } from "react";
import { Badge } from "../../../components/ui/Badge";
import { Button } from "../../../components/ui/button";
import type { CustomMap, MapVisibility } from "../../maps/lib/maps-client";
import { mapThumbnailOptions, mapThumbnailURL } from "../../maps/lib/map-thumbnails";
import { MapThumbnailPickerModal, type ThumbnailCategory } from "./MapThumbnailPickerModal";
import {
  LobbyInput,
  LobbyFieldLabel,
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
          <label className="grid gap-2">
            <LobbyFieldLabel>Map name</LobbyFieldLabel>
            <LobbyInput value={mapName} onChange={(event) => setMapName(event.target.value)} maxLength={80} placeholder="Name your map" disabled={disabled} className="font-semibold" />
          </label>
          <label className="grid gap-2">
          <LobbyFieldLabel>Description <span className="font-regular">(optional)</span></LobbyFieldLabel>
            <LobbyTextarea value={mapDescription} onChange={(event) => setMapDescription(event.target.value)} maxLength={500} placeholder="Tell players what makes this map unique" disabled={disabled} className="min-h-24 resize-none" />
          </label>

          <div className="grid gap-2">
            <LobbyFieldLabel>Difficulty</LobbyFieldLabel>
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
            <LobbyFieldLabel>Visibility</LobbyFieldLabel>
            <LobbySegmentedControl
              value={mapVisibility}
              items={visibilityOptions}
              onChange={(value) => {
                if (!disabled) setMapVisibility(value);
              }}
              className={disabled ? "pointer-events-none opacity-50" : undefined}
            />
          </div>
        </div>

        <div className="grid content-start gap-2">
          <LobbyFieldLabel>Thumbnail</LobbyFieldLabel>
          <Button
            variant="ghost"
            type="button"
            disabled={disabled}
            onClick={() => setThumbnailPickerOpen(true)}
            className="group relative overflow-hidden rounded-xl text-left transition disabled:cursor-not-allowed disabled:opacity-50"
            aria-label={`Choose thumbnail. Current thumbnail: ${selectedThumbnail.label}`}
          >
            <img src={mapThumbnailURL(mapThumbnailKey)} alt="" className="aspect-[16/9] w-full object-cover" />
            <Badge className="absolute bottom-3 left-3 bg-surface-inset/90 text-label text-content-primary">
              {selectedThumbnail.label}
            </Badge>
            <span className="absolute right-3 top-3 inline-flex h-10 w-10 items-center justify-center rounded-full bg-surface-inset/90 text-content-primary transition group-hover:bg-action-primary">
              <Pencil size={17} aria-hidden="true" />
            </span>
          </Button>
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
