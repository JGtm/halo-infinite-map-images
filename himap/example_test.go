package himap_test

import (
	"context"
	"errors"
	"image/png"
	"log"
	"os"

	"github.com/JGtm/halo-infinite-map-images/himap"
)

// Render the top-down background image of a native map from its .module. The caller supplies the
// anchors (world positions of the map objectives): they frame the image and calibrate the play
// level. The map catalogue that knows those positions is not part of this module.
func ExampleCuitCarteNative() {
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
}

// Cook a native map clipped to its navigation mesh. The mesh lives in the map's `any` module,
// the geometry in its `pc` module; ErrSansMaillage means the map has no mesh (two maps do), in
// which case the cooking simply runs without one.
func ExampleChargeMaillageNatif() {
	ctx := context.Background()
	deploy, err := himap.DeployRoot()
	if err != nil {
		log.Fatal(err)
	}
	maillage, bilan, err := himap.ChargeMaillageNatif(ctx, deploy, "ctf_bazaar")
	if err != nil && !errors.Is(err, himap.ErrSansMaillage) {
		log.Fatal(err)
	}
	log.Printf("%d secondary meshes ignored, %d unreadable", bilan.Secondaires, bilan.Illisibles)

	r, _, err := himap.CuitCarteNative(ctx, himap.OptionsCuisson{
		RacineDeploy: deploy,
		CheminModule: deploy + "/pc/levels/multi/ctf_bazaar/ctf_bazaar-rtx-new.module",
		Ancres:       [][3]float64{{-22.8, 0.2, 3.1}, {22.6, 0.2, 3.1}, {0.0, -7.7, 2.3}}, // objectives, world metres
		// Reference surface from the mesh, matter further than 1.5 m from it erased.
		Navmesh: himap.OptionsNavmesh{Maillage: maillage, Rogne: true, Marge: 1.5},
	})
	if err != nil {
		log.Fatal(err)
	}
	// The "plan" look is flat fill plus an outline: it shows what the clipping kept.
	_ = himap.FondPNG(r, r.NX, r.NY, nil, himap.StylePlan)

	// The mesh edges seen by one face only are the walls of the floor plan, in world metres.
	plan, err := himap.PlanDesMurs(maillage)
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range plan.Aretes {
		_ = a // {x1, y1, x2, y2}
	}
	log.Printf("%d wall edges for %d faces", len(plan.Aretes), plan.Faces)
}
