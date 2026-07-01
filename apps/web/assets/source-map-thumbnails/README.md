# Map Thumbnail Sources

Drop source images into the matching directory:

```text
continents/africa.jpg
continents/north-america.png
countries/US.jpg
countries/GB.png
countries/BR_001.jpg
```

Country filenames start with their ISO 3166-1 alpha-2 country code. Continent
filenames use one of these slugs:

```text
africa
antarctica
asia
europe
north-america
oceania
south-america
```

A numeric suffix is optional, so camera-export names such as `BR_001.jpg` and
`africa_001.jpg` work without renaming.

Source images are local and ignored by Git. Generated WebPs are committed and
act as the picker source of truth. The command also generates the shared
frontend/backend catalog at `shared/mapthumbnails/catalog.generated.json`. To
remove an option, delete its generated WebP from
`public/map-thumbnails/continents/` or `public/map-thumbnails/countries/`.

From the repository root, process any dropped images:

```bash
npm run maps:thumbnails
```

Every supported image is cropped to `1280x720`, written as an optimized WebP in
the matching `public/map-thumbnails/` directory, and added to the thumbnail
picker automatically. Supported source formats are JPEG, PNG, WebP, AVIF, and
TIFF.

The five generic sources remain in `generic/` as `variant-1` through `variant-5`.

To verify that every source has a generated output without rewriting files, run:

```bash
npm run maps:thumbnails:check
```
