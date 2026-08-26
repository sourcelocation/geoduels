import { Upload } from "lucide-react";
import { Spinner } from "../../../components/ui/Spinner";
import { useState, type Dispatch, type SetStateAction } from "react";
import type { MapUploadQuota, MapVisibility } from "../../maps/lib/maps-client";
import { MapMetadataFields } from "./MapMetadataFields";
import { MapUploadLimitsModal } from "./MapUploadLimitsModal";
import type { ThumbnailCategory } from "./MapThumbnailPickerModal";
import { Button } from "../../../components/ui/button";
import { FileInputTrigger } from "../../../components/ui/FileInputTrigger";

type MapDifficulty = "easy" | "normal" | "hard";

type MapUploadFormProps = {
  isGuest: boolean;
  mapName: string;
  setMapName: Dispatch<SetStateAction<string>>;
  mapDescription: string;
  setMapDescription: Dispatch<SetStateAction<string>>;
  mapDifficulty: MapDifficulty;
  setMapDifficulty: Dispatch<SetStateAction<MapDifficulty>>;
  mapVisibility: MapVisibility;
  setMapVisibility: Dispatch<SetStateAction<MapVisibility>>;
  mapThumbnailKey: string;
  setMapThumbnailKey: Dispatch<SetStateAction<string>>;
  mapThumbnailCategory: ThumbnailCategory;
  setMapThumbnailCategory: Dispatch<SetStateAction<ThumbnailCategory>>;
  mapThumbnailSearch: string;
  setMapThumbnailSearch: Dispatch<SetStateAction<string>>;
  mapFile: File | null;
  setMapFile: Dispatch<SetStateAction<File | null>>;
  mapUploadError: string;
  quota?: MapUploadQuota;
  setMapUploadError: Dispatch<SetStateAction<string>>;
  uploadPending: boolean;
  onUpload: () => void;
};

export function MapUploadForm({
  isGuest,
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
  mapFile,
  setMapFile,
  mapUploadError,
  quota,
  setMapUploadError,
  uploadPending,
  onUpload,
}: MapUploadFormProps) {
  const [limitsOpen, setLimitsOpen] = useState(false);
  const moderationNote = quota?.restrictedByModeration ? " An active moderation restriction currently forces Base limits." : "";
  const quotaBlockedReason = quota && quota.currentMaps >= quota.maxMaps
    ? `You have reached the ${quota.maxMaps.toLocaleString()} map limit for the ${quota.tier} tier.${moderationNote}`
    : quota && quota.currentActiveLocations >= quota.maxActiveLocations
      ? `You have reached the ${quota.maxActiveLocations.toLocaleString()} active-location limit for the ${quota.tier} tier.${moderationNote}`
      : "";
  const limitError = /limit|rate|throughput|too many/i.test(mapUploadError) ? mapUploadError : "";
  const blockedReason = quotaBlockedReason || limitError;
  const uploadDisabled = isGuest || !!quotaBlockedReason || !mapName.trim() || !mapFile || uploadPending;

  return (
    <>
      <div className="grid gap-4">
        <MapMetadataFields
          disabled={isGuest}
          mapName={mapName}
          setMapName={setMapName}
          mapDescription={mapDescription}
          setMapDescription={setMapDescription}
          mapDifficulty={mapDifficulty}
          setMapDifficulty={setMapDifficulty}
          mapVisibility={mapVisibility}
          setMapVisibility={setMapVisibility}
          mapThumbnailKey={mapThumbnailKey}
          setMapThumbnailKey={setMapThumbnailKey}
          mapThumbnailCategory={mapThumbnailCategory}
          setMapThumbnailCategory={setMapThumbnailCategory}
          mapThumbnailSearch={mapThumbnailSearch}
          setMapThumbnailSearch={setMapThumbnailSearch}
        />
        <div className="grid gap-4">
          <div className="grid gap-2">
            <p className="text-label font-strong text-content-secondary">Upload JSON</p>
            <FileInputTrigger
              accept=".json,application/json"
              disabled={isGuest}
              onChange={(event) => { setMapFile(event.target.files?.[0] || null); setMapUploadError(""); }}
              className="flex min-h-20 flex-col items-center justify-center rounded-lg border border-dashed border-border-strong bg-surface-grouped px-4 text-center text-body-sm font-semibold text-content-secondary hover:border-status-success/50 hover:bg-surface-fill"
            >
              <Upload className="mb-2 text-status-success" size={22} />
              {mapFile ? mapFile.name : "Choose JSON file"}
            </FileInputTrigger>
          </div>

          {mapUploadError ? <p className="text-body-sm font-semibold text-status-danger">{mapUploadError}</p> : null}
          <Button type="button" variant="primary" disabled={uploadDisabled} onClick={onUpload} className="h-11 rounded-xl">
            {uploadPending ? <Spinner size="sm" label="Uploading map" color="current" className="mr-2" /> : <Upload className="mr-2" size={17} />}
            Upload
          </Button>
          {!isGuest ? (
          <Button type="button" variant="ghost" size="sm" onClick={() => setLimitsOpen(true)} className={blockedReason ? "text-status-warning" : undefined}>
            {blockedReason ? "Why?" : "Limits & tiers"}
            </Button>
        ) : null}
        </div>
      </div>
      {limitsOpen ? <MapUploadLimitsModal quota={quota} blockedReason={blockedReason} onClose={() => setLimitsOpen(false)} /> : null}
    </>
  );
}
