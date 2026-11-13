# github.com/JGtm/halo-infinite-map-images

Map-image generator for Halo Infinite: renders the top-down background image of a map from the
geometry stored in the game's own `.module` archives.

## Status

Used in production by LevelUp since October 2026. From v1.0.0 the exported API follows semantic
versioning (a breaking change means a new major version). Published as a Go module in its own
repository; versions are tagged `vX.Y.Z`.

## What it is

One package, `himap`, built on `github.com/JGtm/halo-infinite-files-reader`:

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
go get github.com/JGtm/halo-infinite-map-images@v1.0.0
go get github.com/JGtm/halo-infinite-files-reader@v1.0.0
```

Works from any Go module; no `replace` directive is needed. `himap` imports `github.com/JGtm/halo-infinite-files-reader`, so install both.

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

## Native maps and the navigation mesh

Every native map except `sgh_interlock` and `academy_tutorial` carries the navigation mesh agents
walk on, baked into a resource of its `any` module (the `pc` module holds the render geometry).
`ChargeMaillageNatif(ctx, deploy, "ctf_bazaar")` returns it as a `*hinavmesh.Maillage` plus a
`BilanMaillage` (which resource was decoded, how many secondary meshes were ignored, how many
candidates could not be read); `ErrSansMaillage` means the map has none, and the cooking then
runs without one. Check `BilanMaillage.Illisibles`: when it is not zero the decoded mesh is the
largest *readable* candidate, which may be a secondary one if the real main resource could not
be extracted (a warning is logged).

Both cooking chains take the mesh through one `OptionsNavmesh` (`OptionsCuisson.Navmesh` for
native maps, `OptionsCuissonForge.Navmesh` for Forge maps, whose mesh comes from the variant's
`navmesh.blob` via `hinavmesh.Decode`): the mesh replaces the anchor interpolation as the
reference surface, `Rogne` erases the matter farther than `Marge` metres (zero means 0.5 m, on both
chains) from it in the shape given by `Mode`, `Tolerance` empties cells whose surface is farther than that from the mesh floor, and
`NiveauHaut` keeps the highest level where two overlap. On native maps the clipping runs after
the other erasing steps, so `BilanCuisson.CellulesHorsNavmesh` counts the matter of the final
image that the mesh removed.

`PlanDesMurs(maillage, garde)` returns the floor plan as world-metre segments: the mesh edges that
only one kept face carries (`garde` says, per face, which ones count; nil keeps them all), in a
stable order (an error for an out-of-range vertex index). `PlanRogneAuCadre(plan, calage)` cuts the
segments at the frame of a background image. `StylePlan` renders a `Rendu` as a flat fill outlined
at its silhouette, to judge a clipping; it is a diagnostic look, not the default one.

### Components kept

A navigation mesh can hold slabs that are not the arena: on fifteen Forge maps the largest
component is the canvas floor, not the playable ground. `ComposantesRetenues(maillage, ancres,
matiere)` decides which connected components to keep (two faces are connected as soon as they
share a vertex). A component is kept when:

- it carries at least one objective anchor with a vertical support: the anchor's XY falls inside
  one of its faces and its altitude is within 2 m of that face (`AppuiVerticalAncre`); or
- it covers at least 4 m2 (`AireMinimaleIlot`) and at least 90 % of its area lies on the matter of
  the render when the clipping happens (`PartMinimaleSurMatiere`): a playable islet without any
  objective, such as an isolated weapon pedestal, stays.

If nothing passes (no anchor, no matter), the component with the largest area is kept. The result
(`Retenues`) gives the per-face mask and counts: kept and discarded components, their areas, how
many were kept by anchor and by matter. `BilanCuisson.Navmesh` carries the same counts.

Both cooking chains apply the selection twice. The reference surface is armed before the
geometry is projected, so it only knows the anchors (and the fallback) and reads only the faces of
the kept components; the clipping runs after the projection and chooses again with the render's
matter, which can save islets the reference ignored (they are drawn but not substituted).

### Silhouette mode

`OptionsNavmesh.Mode` shapes the clipping mask. `ModeFaces` (the default) keeps the matter within
`Marge` of the kept faces. `ModeSilhouette` also keeps every hole that cannot be reached from the
border of the grid by walking outside the mask: rocks, pillars and water inside the arena stay
drawn, while a gap open to the outside (a canyon, a bay) stays out. `BilanCuisson.Navmesh` records
the mode and margin that were applied.

The default margin is now 0.5 m on both chains (it was 3 m on the Forge chain): a 3 m margin kept
the rocks and cliffs that border an arena without being part of it. Pass `Marge: 3` for the old
behaviour. `OptionsCuissonForge.CombleAuMaillage` also fills at that default margin.

### Plan image

`PlanPNG(maillage, garde, calage)` draws the exact silhouette of the kept faces (no margin, interior
holes filled as in silhouette mode) as a flat fill, with the border edges as a dark line, on a light
paper. `CalageFondDuRendu(r)` gives the frame of a `Rendu`, so the image has the dimensions and the
calibration of the background, so that the two overlay exactly. The edges are cut at the frame
(`PlanRogneAuCadre`).

Breaking change from v1.0.x: `OptionsCuissonForge` lost `NavmeshReference`, `RogneAuNavmesh`,
`MargeNavmeshCarte`, `ToleranceNavmesh` and `MaillageNiveauHaut`, replaced by `Navmesh`.
`ArmeReferenceDepuisNavmesh` takes the face mask and the high-level flag as parameters and the package
variable `NiveauHautNavmesh` is gone. `PlanDesMurs` returns `(PlanMurs, error)` and takes the face mask.
The default clipping margin went from 3 m to 0.5 m on the Forge chain.

## Requirements

Go 1.26.11 (`go.mod`). cgo is required, transitively: `himodule` of `github.com/JGtm/halo-infinite-files-reader` links the C++
`ooz` decoder, so you need `CGO_ENABLED=1` and a C++ compiler (see the `github.com/JGtm/halo-infinite-files-reader`
README). A local Halo Infinite installation is needed to use the module, not to build it.

## License

AGPL-3.0, the repository license (`LICENSE`, `NOTICE` at the repository root). `himap` depends
on `github.com/JGtm/halo-infinite-files-reader`, which embeds the GPLv3 `ooz` decoder, so a binary that links `himap` is
GPL-3.0-compatible only; see `ooz/NOTICE.md` of `github.com/JGtm/halo-infinite-files-reader`. This is a summary, not legal advice.

The origin of the data files and the rights they do not carry are in [`NOTICE.md`](NOTICE.md).

## Contributing

Issues and pull requests on `github.com/JGtm/LevelUp`, with the path prefix `libs/himap/` in the
title. From the module directory: `go vet ./... && go test ./...`; the tests that read the game
sit behind the `gamefiles` build tag and take tens of minutes (`go test -tags gamefiles`). The
repository convention is French-language code comments and many French identifiers (`Rendu`,
`Ancres`, `Cuit...`), so expect them in the source. Design:
[ADR 0041](../../docs/adr/0041-go-modules-under-libs.md) (modules under `libs/`).

## Not affiliated

Not affiliated with Microsoft, 343 Industries or Halo Studios; use it within the game's terms
of service, on files from your own installation.
