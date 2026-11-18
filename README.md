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
- `CandidatsInstallation` lists the `deploy` folders the game may be in for a store
  (`himap.BoutiqueSteam`: the Steam folder from the Windows registry plus every library of
  `libraryfolders.vdf`; `himap.BoutiqueXbox`: `XboxGames` on each fixed drive). It validates
  nothing and reads no environment variable; a read error comes back next to the candidates
  found. Empty outside Windows.
- `ValideDeploy` checks that a folder is a `deploy` root (`<folder>/<pc|ds|any>/levels/multi`
  exists for at least one variant); the error wraps `ErrDeployInvalide` and says what is missing.

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

To offer the installations of the machine (`ExampleCandidatsInstallation` in the same file):

```go
for _, boutique := range []himap.Boutique{himap.BoutiqueSteam, himap.BoutiqueXbox} {
	candidats, err := himap.CandidatsInstallation(boutique)
	if err != nil {
		log.Printf("%s: partial search: %v", boutique, err)
	}
	for _, c := range candidats {
		if err := himap.ValideDeploy(c); err != nil {
			continue // not installed here, or not a deploy folder
		}
		log.Printf("%s: Halo Infinite found in %s", boutique, c)
	}
}
```

## Native maps and the navigation mesh

Every native map except `sgh_interlock` and `academy_tutorial` carries the navigation mesh agents
walk on, baked into a resource of its `any` module (the `pc` module holds the render geometry).
`ChargeMaillageNatif(ctx, deploy, "ctf_bazaar")` returns it as a `*hinavmesh.Maillage` plus a
`BilanMaillage` (which resource was decoded, how many secondary meshes were ignored, how many
candidates could not be read); `ErrSansMaillage` means the map has none, and the cooking then
runs without one. Check `BilanMaillage.Illisibles`: when it is not zero the decoded mesh is the
largest *readable* candidate, which may be a secondary one if the real main resource could not
be extracted (a warning is logged).

`sgh_interlock` (Live Fire) keeps its geometry in the shared module `pc/globals/common-rtx-new.module`,
and its mesh in the matching `any/globals/common-rtx-new.module`, next to the meshes of neighbouring
arenas. `ChargeMaillageNatifGeometriePartagee(ctx, deploy, "sgh_interlock", geometryModule, anchors)` tries the
map's own `any` module first (a map that carries a mesh gets exactly what `ChargeMaillageNatif` returns),
then, when `geometryModule` lies under `pc/globals/`, picks in the shared module the mesh of the arena
`ChoisitBSP` retains for the geometry: at least half of its vertices inside the arena box, then the most
objective anchors inside its footprint, then the largest share of vertices in the arena, then the smallest
footprint. The other readable meshes count in `BilanMaillage.Secondaires`; `ErrSansMaillage` means none
falls in the arena.

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
matter, which can save islets the reference ignored (they are drawn but have no reference: neither the
substitution nor the tolerance filter touches them). This second selection runs even without
`Rogne`, because it sets the mesh coverage that `CombleAuMaillage` fills.

### Silhouette mode

`OptionsNavmesh.Mode` shapes the clipping mask. `ModeFaces` (the default) keeps the matter within
`Marge` of the kept faces. `ModeSilhouette` also keeps every hole that cannot be reached from the
border of the grid by walking outside the mask: rocks, pillars and water inside the arena stay
drawn, while a gap open to the outside (a canyon, a bay) stays out. `BilanCuisson.Navmesh` records
the mode and margin that were applied.

The default margin is now 0.5 m on both chains (it was 3 m on both): a 3 m margin kept
the rocks and cliffs that border an arena without being part of it. Pass `Marge: 3` for the old
behaviour. `OptionsCuissonForge.CombleAuMaillage` fills with the same resolved margin (`Navmesh.Marge`, or the default).

### Plan image

`PlanPNG(maillage, garde, calage)` draws the height plan of the kept faces. Each face is rasterised
at its altitude (the mean of its vertices) and the top wins per pixel, as seen from the sky. The tint
follows the altitude on a continuous six-step ramp, cold and dark at the bottom, warm and light at the
top (`#60708C #7E8C96 #9EA6A0 #BEBEA0 #DCD6A4 #F6ECB0`), spread between the 2nd and 98th percentiles of
the kept faces' altitudes; a map flatter than 1 m is drawn at the middle of the ramp. The ramp was
checked by Machado 2009 simulation (protanopia, deuteranopia, tritanopia): OKLab lightness is monotone,
and neighbouring steps stay at least 6.7 (x100) apart in each. Do not change it without redoing that
measurement.

A pixel is darkened (a drop shadow) when the pixel 0.7 m to the north-west (at least 2 px) has a top
more than 1.2 m above its own: a step reads as a step. Border edges (seen by one kept face) are drawn
in dark ink, 7 cm thick (at least 1 px); where a level more than 1.5 m higher covers an edge it is
drawn as a grey dashed line instead (6 px drawn, 6 px skipped), a hidden edge. Holes of the mesh
(rocks, pillars) have no face and stay transparent, with their edges drawn.

Output change in v1.3.1: what no kept face covers (the void outside the mesh and its holes) is now
transparent (alpha 0, colour 0) where v1.3.0 painted an opaque light paper (`#F0F1EC`). Faces, shadows
and edges are unchanged pixel for pixel and stay opaque; edges overwrite their pixel and never blend,
so no partial alpha appears. The image is meant to be shown as is, in place of the textured background.

`CalageFondDuRendu(r)` gives the frame of a `Rendu`, so the image has the dimensions and the
calibration of the background, so that the two overlay exactly. Faces and edges are cut at the frame
(`PlanRogneAuCadre` does the same for the wall plan).

### Final clip

When `OptionsNavmesh.Rogne` is armed, the clipping mask (kept components, resolved margin, holes
filled in silhouette mode) is kept and applied a last time at the very end of both chains, after the
water and every layer painted since: matter, assumed ground and water outside the plan silhouette are
removed. Water is posed after the mesh clipping and does not know its mask; without this step it
overflowed the plan view (Deadlock). No selection is redone. `BilanNavmesh.CellulesFinales` and
`EauRetiree` count what the final step removed, and `BilanCuisson.CellulesEau` is lowered by the
water removed.

The plan is the reference mask: when the mesh clipping is armed, the named-zones clipping
(`RogneAuxZones`) never erases a cell inside the plan silhouette. Its effective mask is the union of
the dilated zones and the silhouette; without mesh clipping nothing changes. On the Forge chain the
silhouette is the very mask of the final clip. On the native chain the zones run before the mesh
clipping, so the silhouette is computed just before them from the matter of that moment; the mesh
clipping then chooses the components for good and the final clip uses that definitive mask, which
can differ slightly.
`BilanCuisson.CellulesSauveesParPlan` counts the cells of matter outside the zones that the
silhouette kept (on Catalyst, the south platform that no callout covers).

Breaking change from v1.0.x: `OptionsCuissonForge` lost `NavmeshReference`, `RogneAuNavmesh`,
`MargeNavmeshCarte`, `ToleranceNavmesh` and `MaillageNiveauHaut`, replaced by `Navmesh`.
`ArmeReferenceDepuisNavmesh` takes the face mask and the high-level flag as parameters and the package
variable `NiveauHautNavmesh` is gone. `PlanDesMurs` returns `(PlanMurs, error)` and takes the face mask.
The default clipping margin went from 3 m to 0.5 m on both chains, and the component selection is now
applied (the reference reads only the kept components).

### Relief

`CalculeRelief(ctx, rendu, maillage, garde, calage)` computes the relief of a cooked `Rendu` in the
frame (`CalageFond`) of the background that is *published*, not the one of the cooking: a re-cooking can
come out smaller, larger or shifted, and the published frame is the reference. It returns a `*Relief`:

- `Image`, one pixel per pixel of the frame: `q = R + 256 G` is the height of the top, in steps of 1/256
  game unit from `Donnees.Height.Z0` (the integer part, rounded down, of the lowest height); `B` holds two
  flags (`DrapeauSurCarte`: the height is known and the pixel is in the kept zone; `DrapeauSolMaillage`:
  the height comes from the navigation mesh, not from the geometry); `A` is 255 where the height is known
  and 0 elsewhere (then `R`, `G` and `B` are 0), never anything in between, so that a 2D canvas, which
  stores premultiplied pixels, gives back exactly what was written;
- `Zone`, the kept zone as a mask: the kept faces of the mesh widened by `MargeNavmesh`, narrow passages
  closed, enclosed hollows filled, specks (under `AireMinAnneau`) dropped; without a mesh, the extent of
  the matter;
- `Donnees`, the content of the `.relief.json` file (`DonneesRelief`): the frame the image was projected to
  (a consumer compares it to the frame of the published background: if they differ the relief is stale),
  the height range, the share of the zone whose height comes from the mesh, the degradations as codes;
- `Bilan`, what the computation measured (counts, share, frame offset, matter lost outside the frame).

The top of a pixel is the height of the cooked `Rendu` at the centre of the published pixel (nearest
neighbour, never an average, which would invent a slope on both sides of a wall), completed by the highest
mesh floor where the cooking has no matter at all (natural terrain filled flat in the background) or where
that floor lies more than `ToleranceDessusSol` above the render: the top is never under a walkable floor.
A scale different from the published frame's is refused (`ErrEchelleDifferente`: re-cook the background);
a shifted or larger frame is not an error, its pixels outside the cooked grid have no height from the
render and the `Bilan` counts them. A map without a mesh (`maillage` nil, or no kept face in the frame)
gets `DegradationSansMaillage`. Heights and distances of the relief are game units, about 3 m each, not
the metres the rest of the module says.

`DonneesRelief.Floors` (floors, mesh floors) and `DonneesRelief.Lines` (steps, contours and outline as
polylines) are declared by the type but are filled by later computations of this package, not by
`CalculeRelief`.

`(*Relief).CalculeTraits(ctx)` then fills `DonneesRelief.Lines` from the quantized heights and the kept zone,
in pixels of the published frame: steps where two neighbouring pixels differ by more than `SeuilRupture`
(laid on the lattice between pixels, so that a closed step gives a closed loop and three steps meeting in a
cell share one junction), contour lines every `EquidistanceCourbes` on the heights smoothed over 5 x 5 pixels
where the slope reaches `PenteMinCourbe`, away from any step by more than `RayonSansCourbePresDUneRupture`
pixels (near a step the smoothing mixes both sides and would line every wall with contours), and the outline of the kept zone as closed rings; chained by
increasing node identifier (the output is the same to the byte from one run to the next), specks dropped
(`LongueurMinRupture` per connected group of steps, `LongueurMinCourbe` per contour, `AireMinAnneau` per
ring), simplified by Douglas-Peucker within `ToleranceTraits` pixels and written as tenths of a pixel, the
first point absolute and the next ones as offsets. No line carries a height: a cut by floor filters them
by its own mask, pixel by pixel.

The principles of the relief (height per pixel, shading, contour lines, cut by floor) and the starting
thresholds follow OpenSpartan Cartographer (Den, MIT license); the algorithm is written here, no code of
the original is copied.

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
