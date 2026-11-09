# github.com/JGtm/halo-infinite-map-images

Map-image generator for Halo Infinite: renders the top-down background image of a map from the
geometry stored in the game's own `.module` archives.

This module was extracted from the LevelUp monorepo ([`github.com/JGtm/LevelUp`](https://github.com/JGtm/LevelUp)),
which remains the place where it is developed; released versions are published here.

## Status

Used in production by LevelUp since October 2026. From v1.0.0 the exported API follows semantic versioning: a breaking change means a new major version. Versions are tagged
`vX.Y.Z` in this repository.

## What it is

One package, `himap`, built on [`github.com/JGtm/halo-infinite-files-reader`](https://github.com/JGtm/halo-infinite-files-reader):

- `CuitCarteNative` renders a native map from its `.module` (placed instances, runtime
  geometry, z-buffer and normals seen from above, water, death-volume boundary) and returns the
  `Rendu` plus a `BilanCuisson` that counts what was drawn and lists every degradation.
- `CuitCarteForge` does the same for a Forge map (canvas plus the objects of its `.mvar`,
  clipped to the navigation mesh).
- `FondPNG` encodes a `Rendu` as an image in one of the `StyleFond` looks; `CalageDuRendu` gives
  the pixel -> world mapping to publish with it.
- `NewModuleIndex`, `ReadModuleInstances`, `ReadModuleBSPBounds` and `ReadRuntimeGeoTag` are the
  lower-level readers the renderer is made of, exported for diagnostics.
- `DeployRoot` finds a local installation (`LEVELUP_HALO_DEPLOY`, else the usual locations).

## What it is not

- No map catalogue: the objective positions that frame and calibrate a map (`Ancres`), the
  per-map tuning and the choice of style are yours; this module does not know which map is which.
- No asset pipeline: it returns an image and a `BilanCuisson`; naming, sidecar files, caching
  and publication belong to the caller.
- No game data: it needs your own Halo Infinite installation and ships none of its files.
- Not a replay or analytics layer: it draws geometry, it does not read matches or films.

## Install

```
go get github.com/JGtm/halo-infinite-map-images/himap@v1.0.0
go get github.com/JGtm/halo-infinite-files-reader@v1.0.0
```

Works from any Go module; no `replace` directive is needed. `himap` imports
`github.com/JGtm/halo-infinite-files-reader`, so install both.

## Example

From `himap/example_test.go`, compiled by `go vet` (it does not run without the game):

```go
deploy := "/path/to/Halo Infinite/deploy" // your install's deploy folder, or let himap.DeployRoot discover it
r, bilan, err := himap.CuitCarteNative(context.Background(), himap.OptionsCuisson{
	RacineDeploy: deploy,
	CheminModule: deploy + "/pc/levels/multi/ridgeline/ridgeline-rtx-new.module",
	Ancres:       [][3]float64{{12.5, -40.0, 3.0}, {-60.2, 18.7, 2.5}}, // objectives, world metres
})
if err != nil {
	log.Fatal(err)
}
log.Printf("%d instances drawn, %d degradations", bilan.Dessinees, len(bilan.Degradations))

img := himap.FondPNG(r, r.NX, r.NY, nil, himap.StyleFondParDefaut)
out, err := os.Create("ridgeline.png")
if err != nil {
	log.Fatal(err)
}
defer out.Close()
if err := png.Encode(out, img); err != nil {
	log.Fatal(err)
}
x0, y0, mpp := himap.CalageDuRendu(r) // pixel -> world: x = x0 + (px+0.5)*mpp, y = y0 - (py+0.5)*mpp
log.Printf("origin (%.1f, %.1f), %.4f m/px", x0, y0, mpp)
```

The anchors above are placeholders; real ones come from your own map data.

## Requirements

Go 1.26.10 (`go.mod`). cgo is required, transitively: `himodule` (from `halo-infinite-files-reader`) links the C++
`ooz` decoder, so you need `CGO_ENABLED=1` and a C++ compiler (see the
`halo-infinite-files-reader` README). A local Halo Infinite installation is needed to use the module, not to build it.

## License

AGPL-3.0-only (`LICENSE`). `himap` depends on `halo-infinite-files-reader`, which embeds the GPL-3.0
`ooz` decoder, so a binary that links this module also reaches `ooz` (GPL-3.0) and is
GPL-3.0-compatible only; see the `ooz` notice in `halo-infinite-files-reader`. This is a summary,
not legal advice.

## Contributing

Issues and pull requests on `github.com/JGtm/LevelUp` (the development repository), with the path
prefix `libs/himap/` in the title. Run `go vet ./... && go test ./...`; the tests that read the
game sit behind the `gamefiles` build tag and take tens of minutes (`go test -tags gamefiles`).
The convention of the source is French-language code comments and many French identifiers
(`Rendu`, `Ancres`, `Cuit...`), so expect them.

## Not affiliated

Not affiliated with Microsoft, 343 Industries or Halo Studios; use it within the game's terms
of service, on files from your own installation.
