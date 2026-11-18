//go:build gamefiles

package himap

// sddt_partage_gamefiles_test.go — le sddt de `any/globals/common-rtx-new.module` face a Live
// Fire : les mesures du 2026-11-18 qui fondent sddt_partage.go. Si l'une bouge, la decision de ne
// pas appliquer la frontiere ni l'eau est a refaire.

import (
	"context"
	"path/filepath"
	"testing"
)

// ancresLiveFireCatalogue : les 28 positions d'objectifs de Live Fire (catalogue d'objectifs de
// LevelUp, doublons retires).
var ancresLiveFireCatalogue = [][3]float64{
	{-9.145, 29.672, 0.224}, {-6, 31, 3.7}, {-3.3081, 31.2416, 1.676}, {-2.5692, 30.2, 1.5242},
	{-2.4192, 30.2, 1.5242}, {-2.411, 30.2, 1.5}, {-2.4056, 30.2, 1.5}, {-0.8203, 37.384, 0.2238},
	{-0.6986, 22.545, -0.45}, {6.7837, 33.9556, -0.9914}, {6.7837, 34.5012, -0.9914}, {6.786, 32.9, 1.9},
	{6.8017, 32.6949, 1.8972}, {6.8801, 32.3887, 1.963}, {7.6017, 44.5949, -0.5}, {8.222, 46.3152, -0.0116},
	{13.2, 21.4, -0.8177}, {13.2, 21.4, 0.4}, {13.2, 21.4, 0.4096}, {13.3418, 21.2791, -0.7365},
	{16.053, 43.4724, -0.5011}, {21.88, 42.2, 0.1}, {21.9747, 37.4, 0.9}, {21.9795, 37.4, 0.9},
	{22.5128, 31.813, -0.2155}, {22.9628, 31.513, -0.2155}, {23.9026, 40.0566, 0.11}, {25.2, 37.4, 0.9},
}

func TestModuleVarianteDuModuleCommun(t *testing.T) {
	racine := racineDuJeu(t)
	got, err := ModuleVariante(moduleCommun(racine), "any")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(racine, "any", "globals", "common-rtx-new.module"); got != want {
		t.Errorf("ModuleVariante = %q, attendu %q", got, want)
	}
	// Une carte garde son emplacement.
	pcBazaar := filepath.Join(racine, "pc", "levels", "multi", "ctf_bazaar", "ctf_bazaar-rtx-new.module")
	got, err = ModuleVariante(pcBazaar, "any")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(racine, "any", "levels", "multi", "ctf_bazaar", "ctf_bazaar-rtx-new.module"); got != want {
		t.Errorf("ModuleVariante(Bazaar) = %q, attendu %q", got, want)
	}
}

// Le sddt du module partage : 124 triangles et 42 volumes d'eau, les 28 ancres dans la frontiere,
// 782 faces du maillage sur 786 dedans (les 4 autres au bord est, X > 29,2), l'eau sous le
// plancher du maillage.
func TestSddtDuModuleCommunMesuresLiveFire(t *testing.T) {
	racine := racineDuJeu(t)
	chemin, err := ModuleVariante(moduleCommun(racine), "any")
	if err != nil {
		t.Fatal(err)
	}
	s, err := LitSddt(chemin)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Frontieres) != 124 || len(s.VolumesEau) != 42 {
		t.Errorf("%d triangles-frontieres et %d volumes d'eau, attendu 124 et 42", len(s.Frontieres), len(s.VolumesEau))
	}
	zJeu := MedianeZ(ancresLiveFireCatalogue) - AncrageDecalageSol
	if !FrontiereGardeLesAncres(s, ancresLiveFireCatalogue, zJeu) {
		t.Error("la frontiere exclut des ancres de Live Fire")
	}
	m, _, err := ChargeMaillageNatifGeometriePartagee(context.Background(), racine, "sgh_interlock",
		moduleCommun(racine), ancresLiveFireCatalogue)
	if err != nil {
		t.Fatal(err)
	}
	dehors := 0
	for _, f := range m.Faces {
		var cx, cy, cz float64
		n := float64(len(f.Sommets))
		for _, i := range f.Sommets {
			cx, cy, cz = cx+m.Sommets[i].X/n, cy+m.Sommets[i].Y/n, cz+m.Sommets[i].Z/n
		}
		if !s.ContientFrontiere([3]float64{cx, cy, cz}) {
			dehors++
			if cx < 29.2 {
				t.Errorf("face hors frontiere en (%.2f, %.2f) : attendu au bord est (X > 29,2) seulement", cx, cy)
			}
		}
	}
	if dehors != 4 {
		t.Errorf("%d faces du maillage hors de la frontiere, attendu 4 sur %d", dehors, len(m.Faces))
	}
	for i, v := range s.VolumesEau {
		if v.AABBMax[2] >= m.Min.Z {
			t.Errorf("volume d'eau %d : toit a %.2f m, attendu sous le plancher du maillage (%.2f m)", i, v.AABBMax[2], m.Min.Z)
		}
	}
}

// La cuisson de Live Fire n'applique ni la frontiere ni l'eau du module partage et le dit.
func TestCuissonLiveFireNAppliquePasLeSddtPartage(t *testing.T) {
	if testing.Short() {
		t.Skip("cuisson complete de Live Fire")
	}
	racine := racineDuJeu(t)
	ctx := context.Background()
	geo := moduleCommun(racine)
	m, _, err := ChargeMaillageNatifGeometriePartagee(ctx, racine, "sgh_interlock", geo, ancresLiveFireCatalogue)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := CuitCarteNative(ctx, OptionsCuisson{
		RacineDeploy: racine, CheminModule: geo, Ancres: ancresLiveFireCatalogue,
		Navmesh:     OptionsNavmesh{Maillage: m, Rogne: true},
		EcreteToits: true, PlafondArene: 8, BoiteUtile: [4]float64{-16.7, -10.1, 46.5, 53.7},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.FrontiereAppliquee || b.PlansFrontiere != 0 || b.VolumesEau != 0 || b.CellulesEau != 0 {
		t.Errorf("frontiere appliquee=%v plans=%d, eau volumes=%d cellules=%d : attendu rien",
			b.FrontiereAppliquee, b.PlansFrontiere, b.VolumesEau, b.CellulesEau)
	}
	var f, e bool
	for _, d := range b.Degradations {
		f, e = f || d == raisonFrontierePartagee, e || d == raisonEauPartagee
	}
	if !f || !e {
		t.Errorf("degradations = %q, attendu les deux raisons du module partage", b.Degradations)
	}
}
