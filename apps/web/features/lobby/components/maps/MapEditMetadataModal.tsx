import { Loader2, Save } from "lucide-react";
import { useState } from "react";
import AppModalShell from "../../../../components/ui/AppModalShell";
import type { CustomMap, MapUpdateInput } from "../../../maps/lib/maps-client";
import { MapMetadataFields, thumbnailCategoryFromKey } from "../MapMetadataFields";
import { LobbyActionButton, LobbyNotice } from "../lobby-primitives";

type MapEditMetadataModalProps = {
  map: CustomMap;
  onClose: () => void;
  onSave: (mapId: string, input: MapUpdateInput) => Promise<CustomMap>;
};

export function MapEditMetadataModal({ map, onClose, onSave }: MapEditMetadataModalProps) {
  const [mapName, setMapName] = useState(map.displayName);
  const [mapDescription, setMapDescription] = useState(map.description || "");
  const [mapDifficulty, setMapDifficulty] = useState<CustomMap["difficulty"]>(map.difficulty);
  const [mapVisibility, setMapVisibility] = useState(map.visibility);
  const [mapThumbnailKey, setMapThumbnailKey] = useState(map.thumbnailKey);
  const [mapThumbnailCategory, setMapThumbnailCategory] = useState(thumbnailCategoryFromKey(map.thumbnailKey));
  const [mapThumbnailSearch, setMapThumbnailSearch] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const saveDisabled = saving || !mapName.trim();

  const save = async () => {
    if (saveDisabled) return;
    setSaving(true);
    setError("");
    try {
      await onSave(map.id, {
        displayName: mapName,
        description: mapDescription,
        difficulty: mapDifficulty,
        visibility: mapVisibility,
        thumbnailKey: mapThumbnailKey,
        thumbnailVariant: map.thumbnailVariant,
      });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Map update failed");
    } finally {
      setSaving(false);
    }
  };

  return (
    <AppModalShell
      title="Edit Map"
      onClose={onClose}
      placement="center"
      maxWidthClassName="max-w-[900px]"
      panelClassName="p-4 sm:p-5"
      contentClassName="space-y-4"
    >
      {map.publishedAt ? (
        <LobbyNotice title="Published Map" tone="muted">
          Changes update the public map listing immediately.
        </LobbyNotice>
      ) : null}
      <MapMetadataFields
        disabled={saving}
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
      {error ? <p className="text-xs font-semibold text-red-300">{error}</p> : null}
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <LobbyActionButton type="button" variant="ghost" disabled={saving} onClick={onClose}>
          Cancel
        </LobbyActionButton>
        <LobbyActionButton type="button" disabled={saveDisabled} onClick={save}>
          {saving ? <Loader2 className="mr-2 animate-spin" size={17} /> : <Save className="mr-2" size={17} />}
          Save
        </LobbyActionButton>
      </div>
    </AppModalShell>
  );
}
