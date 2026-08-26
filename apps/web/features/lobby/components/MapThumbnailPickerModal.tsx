import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import { mapThumbnailOptions, mapThumbnailURL } from "../../maps/lib/map-thumbnails";
import { LobbyInput, LobbySegmentedControl } from "./lobby-primitives";

export type ThumbnailCategory = "generic" | "continents" | "countries";

type MapThumbnailPickerModalProps = {
  mapThumbnailCategory: ThumbnailCategory;
  mapThumbnailKey: string;
  mapThumbnailSearch: string;
  onClose: () => void;
  setMapThumbnailCategory: (category: ThumbnailCategory) => void;
  setMapThumbnailKey: (key: string) => void;
  setMapThumbnailSearch: (search: string) => void;
};

export function MapThumbnailPickerModal({
  mapThumbnailCategory,
  mapThumbnailKey,
  mapThumbnailSearch,
  onClose,
  setMapThumbnailCategory,
  setMapThumbnailKey,
  setMapThumbnailSearch,
}: MapThumbnailPickerModalProps) {
  const filteredThumbnailOptions = mapThumbnailOptions.filter((item) => {
    const q = mapThumbnailSearch.trim().toLowerCase();
    return (
      item.category === mapThumbnailCategory &&
      (!q ||
        item.label.toLowerCase().includes(q) ||
        item.search.toLowerCase().includes(q) ||
        item.key.includes(q))
    );
  });
  const categoryOptions: Array<{ value: ThumbnailCategory; label: string }> = [
    { value: "generic", label: "Generic" },
    { value: "continents", label: "Continents" },
    { value: "countries", label: "Countries" },
  ];

  return (
    <AppModalShell
      title="Choose Thumbnail"
      onClose={onClose}
      placement="center"
      maxWidthClassName="max-w-[980px]"
      contentClassName="space-y-4"
    >
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_260px]">
        <LobbySegmentedControl value={mapThumbnailCategory} items={categoryOptions} onChange={setMapThumbnailCategory} />
        <LobbyInput value={mapThumbnailSearch} onChange={(event) => setMapThumbnailSearch(event.target.value)} placeholder="Search thumbnails" className="font-semibold" aria-label="Search thumbnails" />
      </div>
      <div className="grid max-h-[58vh] gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-3">
        {filteredThumbnailOptions.map((option) => (
          <Button
            variant="ghost"
            key={option.key}
            type="button"
            onClick={() => {
              setMapThumbnailKey(option.key);
              onClose();
            }}
            className={`overflow-hidden rounded-lg border text-left transition ${mapThumbnailKey === option.key ? "border-status-success bg-status-success/10" : "border-border-default bg-surface-grouped hover:bg-surface-fill"}`}
          >
            <img src={mapThumbnailURL(option.key)} alt="" className="aspect-[16/9] w-full object-cover" />
            <div className="p-2 text-label font-strong text-content-primary">{option.label}</div>
          </Button>
        ))}
      </div>
    </AppModalShell>
  );
}
