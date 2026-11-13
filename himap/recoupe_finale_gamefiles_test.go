//go:build gamefiles

package himap

// recoupe_finale_gamefiles_test.go - l'etape finale sur le jeu installe : Deadlock (btb_drydock),
// la carte qui a montre l'eau depassant de la vue plan. Mesures epinglees du 2026-11-14.

import (
	"context"
	"testing"
)

// ancresDeadlock : positions monde des objectifs de Deadlock et de Deadlock Heavies (catalogue de
// LevelUp, doublons retires). Elles cadrent l'image et calent le niveau de jeu.
var ancresDeadlock = [][3]float64{
	{38.5, 93, 72.7}, {-1.8, 36.6, 75.4}, {11.348, 105.9688, 76.1}, {36.3998, 27.5132, 74.1},
	{24, 62.5, 75.2}, {55.4314, 52.0649, 77.6805}, {-10.8119, 83.1198, 79.881},
	{55.4321, 52.0643, 77.6872}, {36.5457, 27.1363, 74.0934}, {57.9358, 73.5143, 75.9663},
	{-10.8114, 83.1202, 79.8875}, {10.8883, 106.5317, 76.0974}, {-8.0575, 63.2101, 78.8956},
	{22.522, 63.881, 75.2415}, {54.8344, 52.4872, 78.5613}, {-10.2835, 82.7486, 80.7005},
	{55.4089, 53.207, 77.6805}, {55.1424, 52.825, 77.6807}, {54.8775, 52.4419, 77.6862},
	{54.6119, 52.0519, 77.6856}, {54.347, 51.666, 77.6842}, {-10.8277, 81.9687, 79.8751},
	{-10.5618, 82.3577, 79.8756}, {-10.2912, 82.7492, 79.8752}, {-10.0242, 83.1398, 79.8751},
	{-9.7593, 83.5323, 79.8737}, {56.6573, 83.8762, 74.7386}, {22.0587, 99.9879, 74.734},
	{41.3294, 97.9772, 72.1674}, {-12.5159, 54.9955, 78.837}, {18.2978, 34.1515, 76.4573},
	{-2.0325, 36.295, 74.7012}, {35.5187, 59.8271, 75.794}, {28.2121, 40.9351, 74.168},
	{0.0433, 58.8865, 75.406}, {28.0591, 90.9453, 75.1116}, {32.2362, 70.2766, 76.5595},
	{47.753, 73.2457, 74.4075}, {36.0634, 85.8557, 73.2331}, {22.9993, 63.5927, 75.2081},
	{9.3923, 49.8728, 75.7178}, {38.4934, 92.9848, 72.6674}, {-1.8115, 36.5882, 75.3674},
	{11.0945, 105.7871, 76.0674}, {36.8878, 27.6848, 74.0674}, {22.5909, 63.8861, 75.1674},
	{10.53, 31.7144, 77.75}, {49.89, 86.8391, 74.97}, {40.1371, 64.5992, 76.1585},
	{-7.8383, 40.0275, 77.8806}, {25.29, 96.0575, 75.24}, {4.9984, 76.7773, 78.878},
	{58.2683, 83.4323, 74.2204}, {14.8494, 96.5456, 76.3706},
}

// Deadlock : l'eau est posee apres le rognage ; l'etape finale la recoupe a la silhouette du plan.
// Apres elle, ni eau visible ni matiere hors du masque ; avant, de l'eau depassait.
func TestRecoupeFinaleDeadlock(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	maillage, _, err := ChargeMaillageNatif(ctx, racine, "btb_drydock")
	if err != nil {
		t.Fatal(err)
	}
	r, b, err := CuitCarteNative(ctx, OptionsCuisson{
		RacineDeploy: racine,
		CheminModule: moduleDuJeu(t, "pc", "btb_drydock"),
		Ancres:       ancresDeadlock,
		CibleCadrePx: CibleCadrePx,
		Navmesh:      OptionsNavmesh{Maillage: maillage, Rogne: true, Mode: ModeSilhouette, Marge: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("deadlock : %d x %d px, %d volumes d'eau, eau restante %d, eau retiree %d, matiere retiree %d",
		r.NX, r.NY, b.VolumesEau, b.CellulesEau, b.Navmesh.EauRetiree, b.Navmesh.CellulesFinales)
	if b.Navmesh.EauRetiree <= 0 {
		t.Errorf("l'etape finale n'a retire aucune cellule d'eau (%d)", b.Navmesh.EauRetiree)
	}
	for j := 0; j < r.NY; j++ {
		for i := 0; i < r.NX; i++ {
			k := j*r.NX + i
			if r.masqueFinal[k] {
				continue
			}
			if r.Eau(i, j) || r.matiereEn(r.Min[0]+(float64(i)+0.5)*r.Cell, r.Min[1]+(float64(j)+0.5)*r.Cell) {
				t.Fatalf("cellule (%d, %d) hors de la silhouette : eau ou matiere restante", i, j)
			}
		}
	}
	// Mesure du 2026-11-14 : 407 176 cellules d eau retirees sur 425 172 posees.
	const eauRetiree, matiereFinale, eauRestante = 407176, 0, 17996
	if b.Navmesh.EauRetiree != eauRetiree || b.Navmesh.CellulesFinales != matiereFinale || b.CellulesEau != eauRestante {
		t.Errorf("eau retiree %d, matiere retiree %d, eau restante %d ; attendu %d, %d et %d",
			b.Navmesh.EauRetiree, b.Navmesh.CellulesFinales, b.CellulesEau, eauRetiree, matiereFinale, eauRestante)
	}
}
