//go:build gamefiles

package himap

// relief_sols_gamefiles_test.go — les sols et les etages de Recharge (sgh_blueprint), cuits depuis
// l'installation du jeu et projetes dans le calage du fond PUBLIE, comme relief_gamefiles_test.go.
// Mesures du 2026-11-18.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

// Recharge a trois niveaux (conception §4.5 : 0,00 ; 2,21 ; 4,40 u ; mesure du 2026-11-18 : 0,00 ; 2,21 ;
// 4,40, a la lettre). La tolerance de 0,05 u laisse a la mediane des echantillons autour d'un pic le bruit
// d'un sol interpole (les trois sols ne sont pas parfaitement plats) sans laisser passer un etage voisin :
// les etages sont a plus de 2,2 u l'un de l'autre.
func TestReliefSolsRecharge(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	maillage, _, err := ChargeMaillageNatif(ctx, racine, "sgh_blueprint")
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := CuitCarteNative(ctx, OptionsCuisson{
		RacineDeploy: racine,
		CheminModule: moduleDuJeu(t, "pc", "sgh_blueprint"),
		Ancres:       ancresRecharge,
		Echelle:      0.029,
		EcreteToits:  true,
		PlafondArene: 4,
		SansEau:      true,
		Navmesh:      OptionsNavmesh{Maillage: maillage, Rogne: true, Mode: ModeSilhouette},
	})
	if err != nil {
		t.Fatal(err)
	}
	matiere := func(x, y float64) bool {
		i, j := int(math.Floor((x-r.Min[0])/r.Cell)), int(math.Floor((y-r.Min[1])/r.Cell))
		_, ok := r.Altitude(i, j)
		return ok
	}
	retenues := ComposantesRetenues(maillage, ancresRecharge, matiere)
	rel, err := CalculeRelief(ctx, r, maillage, retenues.Garde, calageRechargePublie)
	if err != nil {
		t.Fatal(err)
	}
	debut := time.Now()
	if err := rel.CalculeSols(ctx, maillage, retenues.Garde); err != nil {
		t.Fatal(err)
	}
	duree := time.Since(debut)
	f := rel.Donnees.Floors
	j, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	w, _ := gzip.NewWriterLevel(&gz, 5)
	_, _ = w.Write(j)
	_ = w.Close()
	t.Logf("sols en %s ; etages %v ; min %.2f max %.2f ; %d sommets, %d faces ; floors %d octets, %d apres gzip-5 ; degradations %v",
		duree, f.Niveaux, *f.Min, *f.Max, len(f.Sommets)/3, len(f.Faces), len(j), gz.Len(), rel.Donnees.Degradations)

	const tolerance = 0.05
	want := []float64{0.00, 2.21, 4.40}
	if len(f.Niveaux) != len(want) {
		t.Fatalf("etages %v, attendu %v (conception §4.5)", f.Niveaux, want)
	}
	for i := range want {
		if math.Abs(f.Niveaux[i]-want[i]) > tolerance {
			t.Errorf("etage %d a %.2f u, attendu %.2f a %.2f u pres", i, f.Niveaux[i], want[i], tolerance)
		}
	}
	if math.Abs(*f.Min+0.01) > tolerance || math.Abs(*f.Max-4.42) > tolerance {
		t.Errorf("min %.2f max %.2f, attendu -0,01 et 4,42 a %.2f u pres", *f.Min, *f.Max, tolerance)
	}
	verifieCoherence(t, f)
	if len(rel.Donnees.Degradations) != 0 {
		t.Errorf("degradations %v, attendu aucune", rel.Donnees.Degradations)
	}
	// La conception mesure 21,0 Ko de sols pour Recharge (8,3 Ko apres gzip) en polygones ; ici chaque face
	// est un triangle (mesure : 27,5 Ko, 10,1 Ko apres gzip). On tient 1,5 fois la conception : au-dela, le
	// format a derive.
	if len(j) > 21*1024*3/2 || gz.Len() > 83*1024/10*3/2 {
		t.Errorf("floors pese %d octets (%d apres gzip), au-dela de 1,5 fois la conception (21,0 Ko, 8,3 Ko)", len(j), gz.Len())
	}
}
