# Country Map Import Pipeline

Use this pipeline to generate country maps from a local Vali install and import
them as official GeoDuels maps.

## 1. Generate Vali Outputs

Prepare a Vali config template that works for one country. The batch script
overrides `countryCodes` and `distributionStrategy.locationCountGoal` for each
country directory under `--source-root`.

```bash
npm run maps:vali:generate -- \
  --template datasets-config/country.json \
  --vali-bin /path/to/vali \
  --source-root /path/to/vali/downloaded/countries \
  --output-root datasets/generated/countries
```

By default, broad-coverage countries target 10,000 locations and smaller
countries target 2,000. Override exact goals when needed:

```bash
npm run maps:vali:generate -- \
  --template datasets-config/country.json \
  --source-root /path/to/vali/downloaded/countries \
  --output-root datasets/generated/countries \
  --target-counts datasets/generated/country-targets.json
```

The run is sequential and resumable through
`datasets/generated/countries/.progress.jsonl`. It writes
`datasets/generated/countries/country-maps.manifest.json` as maps complete.

## 2. Validate Street View Panoramas

Run the existing Street View metadata validator on generated files before
production import. Metadata requests do not load Street View imagery.

```bash
cd apps/web
GOOGLE_MAPS_API_KEY='server-key' npm run validate:streetview -- \
  --input ../../datasets/generated/countries/FR/france.json \
  --output ../../datasets/generated/countries/FR/france.clean.json
```

Update the manifest file paths to point at the cleaned outputs.

## 3. Dry-Run The Import

The import script validates every manifest entry locally, checks thumbnail keys
against the generated thumbnail catalog, and reports valid/rejected location
counts. It does not write by default.

```bash
npm run maps:country-import -- \
  --manifest datasets/generated/countries/country-maps.manifest.json \
  --report datasets/generated/countries/import-report.json
```

Fix any `blocked` entries before importing.

## 4. Import To Production

Use an admin app access token. Production writes require both `--import` and
`--confirm-production`.

```bash
GEODUELS_ADMIN_ACCESS_TOKEN='admin-access-jwt' \
npm run maps:country-import -- \
  --manifest datasets/generated/countries/country-maps.manifest.json \
  --api-base https://geoduels.io \
  --import \
  --confirm-production \
  --report datasets/generated/countries/import-report.prod.json
```

Each map is posted to `/v1/admin/maps/official/import` as multipart form data.
The API upserts a deterministic official map key such as `country/france`,
replaces its current locations atomically, marks it public and official, and
sets `officialRegion` to `country:FR`.

## 5. Verify

After import:

- Check the report for every API response.
- Open the map browser and filter official maps.
- Spot-check thumbnails and country names.
- Launch a private match against several imported maps.
