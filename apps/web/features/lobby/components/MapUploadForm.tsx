import { Loader2, Upload } from "lucide-react";
import { useState, type Dispatch, type SetStateAction } from "react";
import type { MapUploadQuota, MapVisibility } from "../../maps/lib/maps-client";
import { MapMetadataFields } from "./MapMetadataFields";
import { MapUploadLimitsModal } from "./MapUploadLimitsModal";
import type { ThumbnailCategory } from "./MapThumbnailPickerModal";
import {
  LobbyActionButton,
} from "./lobby-primitives";

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
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">Upload JSON</p>
            <label className="flex min-h-20 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed border-white/20 bg-black/20 px-4 text-center text-sm font-semibold text-[#a9bfd4] hover:border-[#2ad18f]/50">
              <Upload className="mb-2 text-[#2ad18f]" size={22} />
              {mapFile ? mapFile.name : "Choose JSON file"}
              <input type="file" accept=".json,application/json" className="hidden" disabled={isGuest} onChange={(event) => { setMapFile(event.target.files?.[0] || null); setMapUploadError(""); }} />
            </label>
          </div>

          {mapUploadError ? <p className="text-xs font-semibold text-red-300">{mapUploadError}</p> : null}
          <LobbyActionButton type="button" disabled={uploadDisabled} onClick={onUpload} className="h-11 rounded-xl">
            {uploadPending ? <Loader2 className="mr-2 animate-spin" size={17} /> : <Upload className="mr-2" size={17} />}
            Upload
          </LobbyActionButton>
          {!isGuest ? (
            <button type="button" onClick={() => setLimitsOpen(true)} className={`text-xs font-bold transition hover:text-white ${blockedReason ? "text-amber-200" : "text-[#6f8998]"}`}>
            {blockedReason ? "Why?" : "Limits & tiers"}
          </button>
        ) : null}
        </div>
      </div>
      {limitsOpen ? <MapUploadLimitsModal quota={quota} blockedReason={blockedReason} onClose={() => setLimitsOpen(false)} /> : null}
    </>
  );
}
