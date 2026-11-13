//go:build gamefiles

package himap

// silhouette_gamefiles_test.go — le mode silhouette et la selection des composantes sur le jeu
// installe : Bazaar (cuisson complete), Chasm (maillage natif a ilots) et Isolation (blob Forge
// des donnees de test de la brique lecteur). Les mesures epinglees sont celles du 2026-11-13.

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

func TestSilhouetteBazaarMarge05(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	opts := OptionsCuisson{
		RacineDeploy: racine,
		CheminModule: moduleDuJeu(t, "pc", "ctf_bazaar"),
		Ancres:       ancresBazaar,
		Echelle:      0.034,
	}
	rSans, _, err := CuitCarteNative(ctx, opts)
	if err != nil {
		t.Fatalf("cuisson sans maillage : %v", err)
	}
	maillage, _, err := ChargeMaillageNatif(ctx, racine, "ctf_bazaar")
	if err != nil {
		t.Fatal(err)
	}
	cuit := func(mode ModeNavmesh) (*Rendu, BilanCuisson) {
		o := opts
		o.Navmesh = OptionsNavmesh{Maillage: maillage, Rogne: true, Mode: mode, Marge: 0.5}
		r, b, err := CuitCarteNative(ctx, o)
		if err != nil {
			t.Fatalf("cuisson en mode %q : %v", mode, err)
		}
		return r, b
	}
	rFaces, bFaces := cuit(ModeFaces)
	rSil, bSil := cuit(ModeSilhouette)

	sans, faces, sil := matiereDuRendu(rSans), matiereDuRendu(rFaces), matiereDuRendu(rSil)
	partFaces, partSil := 1-float64(faces)/float64(sans), 1-float64(sil)/float64(sans)
	t.Logf("matiere : %d sans maillage, %d en mode faces (part rognee %.2f %%), %d en silhouette (%.2f %%) ; composantes %+v",
		sans, faces, 100*partFaces, sil, 100*partSil, bSil.Navmesh)

	if bSil.Ancres != 17 || bSil.AncresAvecSol != 17 {
		t.Errorf("%d ancres sur %d au sol, attendu 17/17", bSil.AncresAvecSol, bSil.Ancres)
	}
	if bSil.Navmesh.Mode != ModeSilhouette || bSil.Navmesh.Marge != 0.5 || bFaces.Navmesh.Mode != ModeFaces {
		t.Errorf("mode et marge au bilan : %q a %g m (silhouette), %q (faces)", bSil.Navmesh.Mode, bSil.Navmesh.Marge, bFaces.Navmesh.Mode)
	}
	if sil < faces {
		t.Errorf("la silhouette garde %d cellules, moins que le mode faces (%d)", sil, faces)
	}
	// Mesure du 2026-11-13 : 62,18 pour cent de la matiere rognee en silhouette a 0,5 m (62,52 en
	// mode faces, la silhouette garde donc 0,3 point de plus), a deux points pres.
	const partMesureeSilhouette, tolerance = 0.6218, 0.02
	if math.Abs(partSil-partMesureeSilhouette) > tolerance {
		t.Errorf("part rognee en silhouette = %.4f, attendu %.4f a %.2f pres", partSil, partMesureeSilhouette, tolerance)
	}
	// Composantes (2026-11-13) : 17 dont 5 gardees, 4 par une ancre appuyee et 1 par la matiere
	// seule ; 1 011,8 m2 gardes pour 21,8 m2 ecartes.
	n := bSil.Navmesh
	if n.Gardees != 5 || n.Ecartees != 12 || n.ParAncre != 4 || n.ParMatiere != 1 || n.Repli {
		t.Errorf("composantes %+v, attendu 5 gardees (4 par ancre, 1 par matiere) et 12 ecartees", n)
	}
	if math.Abs(n.AireGardee-1011.76) > 10 || math.Abs(n.AireEcartee-21.82) > 1 {
		t.Errorf("aires %.2f gardee et %.2f ecartee, attendu 1011,76 et 21,82 m2", n.AireGardee, n.AireEcartee)
	}

	// Le plan des faces gardees a les dimensions du fond et porte aplat et trait.
	plan, err := PlanPNG(maillage, nil, CalageFondDuRendu(rSil))
	if err != nil {
		t.Fatal(err)
	}
	if bornes := plan.Bounds(); bornes.Dx() != rSil.NX || bornes.Dy() != rSil.NY {
		t.Errorf("plan de %d x %d, fond de %d x %d", bornes.Dx(), bornes.Dy(), rSil.NX, rSil.NY)
	}
	aplat, trait := 0, 0
	for py := 0; py < rSil.NY; py++ {
		for px := 0; px < rSil.NX; px++ {
			switch plan.RGBAAt(px, py) {
			case teintePlanAplat:
				aplat++
			case teintePlanContour:
				trait++
			}
		}
	}
	t.Logf("plan de Bazaar sur le cadre du fond : %d pixels d'aplat, %d de trait", aplat, trait)
	if aplat == 0 || trait == 0 {
		t.Errorf("plan sans aplat (%d) ou sans trait (%d)", aplat, trait)
	}
}

func TestComposantesRetenuesChasm(t *testing.T) {
	maillage, _, err := ChargeMaillageNatif(context.Background(), racineDuJeu(t), "chasm")
	if err != nil {
		t.Fatal(err)
	}
	comps := hinavmesh.Composantes(maillage)
	res := ComposantesRetenues(maillage, nil, nil)
	t.Logf("chasm : %d composantes, la plus grande %.1f m2 ; sans ancre ni matiere : %d gardee, %d ecartees", len(comps), comps[0].Aire, res.Gardees, res.Ecartees)
	// Mesure du 2026-11-13 : 50 composantes, la plus grande de 643,5 m2. Aucune ancre de Chasm
	// n'est versionnee ici : sans ancre ni matiere la selection retombe sur la plus grande.
	if len(comps) != 50 || math.Abs(comps[0].Aire-643.47) > 1 {
		t.Errorf("%d composantes dont la plus grande de %.2f m2, attendu 50 et 643,47", len(comps), comps[0].Aire)
	}
	if !res.Repli || res.Gardees != 1 || res.Ecartees != 49 {
		t.Errorf("repli attendu sur la plus grande : %d gardee, %d ecartees, repli %v", res.Gardees, res.Ecartees, res.Repli)
	}
}

// moduleLecteur rend le dossier du module du lecteur de fichiers, ou saute le test.
func moduleLecteur(t *testing.T) string {
	t.Helper()
	sortie, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/JGtm/halo-infinite-files-reader").Output()
	if err != nil {
		t.Skipf("go list indisponible : %v", err)
	}
	return strings.TrimSpace(string(sortie))
}

// Isolation : le blob Forge des donnees de test du lecteur. La brique n'en versionne aucun ;
// il est lu dans le cache des modules. Aucune ancre d'Isolation n'est disponible ici : la
// selection retombe sur le repli (la plus grande composante).
func TestComposantesRetenuesIsolation(t *testing.T) {
	chemin := filepath.Join(moduleLecteur(t), "hinavmesh", "testdata", "01af558d-53ab-4f05-ba68-92d805fc6260.navmesh.blob")
	blob, err := os.ReadFile(chemin)
	if err != nil {
		t.Skipf("blob de test absent : %v", err)
	}
	maillage, err := hinavmesh.Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	res := ComposantesRetenues(maillage, nil, nil)
	t.Logf("isolation : %d gardee (%.1f m2), %d ecartees (%.1f m2)", res.Gardees, res.AireGardee, res.Ecartees, res.AireEcartee)
	// Memes valeurs que les mesures du lecteur : 17 composantes, 1 427 m2 gardes, 772,6 m2 ecartes.
	if !res.Repli || res.Gardees != 1 || res.Ecartees != 16 || math.Abs(res.AireEcartee-772.6) > 8 {
		t.Errorf("repli attendu : 1 gardee et 16 ecartees (772,6 m2), obtenu %d, %d, %.1f m2", res.Gardees, res.Ecartees, res.AireEcartee)
	}
}
