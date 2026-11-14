package himap

import (
	"context"
	"math"
	"testing"
)

// zonesPremieresColonnes : un polygone de zone qui ne couvre que x < 3 m de la grille de
// renduPleinDeMatiere.
var zonesPremieresColonnes = [][][2]float64{{{0, 0}, {3, 0}, {3, 10}, {0, 10}}}

// optionsDuCablage : rognage au maillage en silhouette sur l'anneau, zones rognees sans dilatation.
func optionsDuCablage() OptionsNavmesh {
	return OptionsNavmesh{Maillage: anneauMaillage(), Rogne: true, Mode: ModeSilhouette, Marge: 0.2,
		ancres: [][3]float64{{3, 3, 0}}}
}

func matiereEn(r *Rendu, i, j int) bool { return !math.IsInf(r.z[j*r.NX+i], -1) }

// poseDeLEauHorsDuPlan pose de l'eau dans le coin (0, 0), hors de la silhouette de l'anneau.
func poseDeLEauHorsDuPlan(r *Rendu) {
	r.eau = make([]bool, r.NX*r.NY)
	r.eau[0] = true
}

// La fin de la chaine Forge, jouee telle que CuitCarteForge l'appelle : la silhouette du plan
// sauve la matiere hors des zones, et l'etape finale recoupe ce qui a ete peint entre-temps.
// Mutations jouees a la main : retirer l'etape finale de finitRenduForge, ou l'union zones +
// silhouette de rogneAuxZonesEtSilhouette, fait echouer ce test.
func TestFinitRenduForgeUnionZonesSilhouetteEtEtapeFinale(t *testing.T) {
	r := renduPleinDeMatiere()
	poseDeLEauHorsDuPlan(r)
	var b BilanCuisson
	opts := OptionsCuissonForge{Navmesh: optionsDuCablage(), RogneAuxZones: true, MargeZones: -1}
	finitRenduForge(context.Background(), r, &b, opts, zonesPremieresColonnes)
	if !matiereEn(r, 5, 5) {
		t.Error("la matiere dans la silhouette, hors des zones, a ete effacee : l'union zones + silhouette manque")
	}
	if b.CellulesSauveesParPlan == 0 {
		t.Error("aucune cellule sauvee par le plan au bilan")
	}
	if r.Eau(0, 0) {
		t.Error("l'eau hors de la silhouette est restee : l'etape finale manque")
	}
	if matiereEn(r, 9, 9) {
		t.Error("la matiere hors de la silhouette est restee")
	}
}

// La chaine native : la silhouette est posee AVANT les zones. Posee apres elles, les zones
// effaceraient la matiere du plan hors de leur emprise sans retour possible. L'etape finale
// (apres l'eau) recoupe l'eau.
// Mutations jouees a la main : etapePlan deplace apres les zones dans rogneLeRenduNatif, ou
// etapeFinale retiree de finitRenduNatif, font echouer ce test.
func TestChaineNativeSilhouetteAvantLesZonesEtEtapeFinale(t *testing.T) {
	ctx := context.Background()
	r := renduPleinDeMatiere()
	var b BilanCuisson
	opts := OptionsCuisson{Navmesh: optionsDuCablage(), ZonesNommees: zonesPremieresColonnes,
		RogneAuxZones: true, MargeZones: -1, SansEau: true}
	rogneLeRenduNatif(ctx, r, &b, opts)
	if !matiereEn(r, 5, 5) {
		t.Error("la matiere dans la silhouette, hors des zones, a ete effacee : la silhouette n'est pas posee avant les zones")
	}
	if b.CellulesSauveesParPlan == 0 {
		t.Error("aucune cellule sauvee par le plan au bilan")
	}
	poseDeLEauHorsDuPlan(r)
	finitRenduNatif(ctx, r, &b, opts)
	if r.Eau(0, 0) {
		t.Error("l'eau hors de la silhouette est restee : l'etape finale manque")
	}
	if !matiereEn(r, 5, 5) {
		t.Error("l'etape finale a efface la matiere du plan")
	}
}
