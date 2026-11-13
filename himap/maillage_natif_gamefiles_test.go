//go:build gamefiles

package himap

// maillage_natif_gamefiles_test.go — le maillage de navigation des cartes natives, lu dans le
// jeu installe : ChargeMaillageNatif, PlanDesMurs et la cuisson de Bazaar avec rognage au
// maillage. Les mesures epinglees sont celles du 2026-11-13.

import (
	"context"
	"errors"
	"math"
	"testing"
)

// racineDuJeu rend la racine `deploy` de l'installation, ou saute le test.
func racineDuJeu(t *testing.T) string {
	t.Helper()
	racine, err := DeployRoot()
	if err != nil {
		t.Skipf("installation du jeu introuvable : %v", err)
	}
	return racine
}

func TestChargeMaillageNatifMesuresDeReference(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	cas := []struct {
		module              string
		sommets, faces      int
		secondaires         int
		attenduSansMaillage bool
	}{
		{module: "ctf_bazaar", sommets: 2312, faces: 670, secondaires: 0},
		{module: "chasm", secondaires: 5},
		{module: "sgh_interlock", attenduSansMaillage: true},
	}
	for _, c := range cas {
		t.Run(c.module, func(t *testing.T) {
			m, bilan, err := ChargeMaillageNatif(ctx, racine, c.module)
			if c.attenduSansMaillage {
				if !errors.Is(err, ErrSansMaillage) {
					t.Fatalf("erreur = %v, attendu ErrSansMaillage", err)
				}
				if m != nil {
					t.Error("maillage non nil avec ErrSansMaillage")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s : %d sommets, %d faces, bilan %+v", c.module, len(m.Sommets), len(m.Faces), bilan)
			if c.sommets > 0 && (len(m.Sommets) != c.sommets || len(m.Faces) != c.faces) {
				t.Errorf("%d sommets et %d faces, attendu %d et %d", len(m.Sommets), len(m.Faces), c.sommets, c.faces)
			}
			if bilan.Secondaires != c.secondaires {
				t.Errorf("secondaires = %d, attendu %d", bilan.Secondaires, c.secondaires)
			}
			if bilan.Octets == 0 {
				t.Error("bilan.Octets nul")
			}
		})
	}
}

func TestChargeMaillageNatifModuleInconnu(t *testing.T) {
	_, _, err := ChargeMaillageNatif(context.Background(), racineDuJeu(t), "carte_qui_n_existe_pas")
	if err == nil || errors.Is(err, ErrSansMaillage) {
		t.Fatalf("erreur = %v, attendu une erreur d'ouverture distincte de ErrSansMaillage", err)
	}
}

// ancresBazaar : positions monde des objectifs de Bazaar (catalogue de LevelUp, doublons
// retires). Elles cadrent l'image et calent le niveau de jeu.
var ancresBazaar = [][3]float64{
	{0.0265, -7.6504, 2.2889}, {-10.8065, -8.8143, 3.2041}, {17.6000, 6.5112, 1.1000},
	{-11.2447, -0.0546, 0.0782}, {6.5000, -5.3000, -0.1848}, {-22.8000, 0.2000, 3.1000},
	{22.5750, 0.1944, 3.1000}, {-0.0243, 2.6347, 1.4748}, {-9.8271, -4.5200, 3.2000},
	{9.9000, -4.7058, 3.1853}, {22.8000, 0.1968, 3.1500}, {-23.0000, 0.1739, 3.1500},
	{-0.0072, -11.5949, -2.1000}, {-0.0256, 2.6160, 1.4672}, {-12.7208, 7.1589, 0.9908},
	{12.7289, 7.1196, 0.9492}, {-0.0110, -11.5841, -2.1853},
}

// matiereDuRendu compte les cellules qui portent une surface.
func matiereDuRendu(r *Rendu) int {
	n := 0
	for j := 0; j < r.NY; j++ {
		for i := 0; i < r.NX; i++ {
			if _, ok := r.Altitude(i, j); ok {
				n++
			}
		}
	}
	return n
}

func TestCuisineBazaarAvecMaillageNatif(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	opts := OptionsCuisson{
		RacineDeploy: racine,
		CheminModule: moduleDuJeu(t, "pc", "ctf_bazaar"),
		Ancres:       ancresBazaar,
		Echelle:      0.034,
	}
	rSans, bSans, err := CuitCarteNative(ctx, opts)
	if err != nil {
		t.Fatalf("cuisson sans maillage : %v", err)
	}
	maillage, _, err := ChargeMaillageNatif(ctx, racine, "ctf_bazaar")
	if err != nil {
		t.Fatal(err)
	}
	opts.Navmesh = OptionsNavmesh{Maillage: maillage, Rogne: true, Marge: 1.5}
	rAvec, bAvec, err := CuitCarteNative(ctx, opts)
	if err != nil {
		t.Fatalf("cuisson avec maillage : %v", err)
	}

	if d := math.Abs(bAvec.NiveauDeJeu - bSans.NiveauDeJeu); d > 0.2 {
		t.Errorf("niveau de jeu %.3f m avec maillage, %.3f m sans : ecart %.3f m > 0,2 m",
			bAvec.NiveauDeJeu, bSans.NiveauDeJeu, d)
	}
	if bAvec.AncresAvecSol != bAvec.Ancres {
		t.Errorf("%d ancres sur %d au sol (%d dans le cadre), attendu toutes", bAvec.AncresAvecSol, bAvec.Ancres, bAvec.AncresDansLeCadre)
	}
	sans, avec := matiereDuRendu(rSans), matiereDuRendu(rAvec)
	part := 1 - float64(avec)/float64(sans)
	t.Logf("matiere %d cellules sans maillage, %d avec ; part rognee %.2f %% ; effacees hors maillage %d ; ancres %d/%d ; ecart ancres %.3f m (sans : %.3f m)",
		sans, avec, 100*part, bAvec.CellulesHorsNavmesh, bAvec.AncresAvecSol, bAvec.Ancres, bAvec.EcartMedianAncre, bSans.EcartMedianAncre)
	if bAvec.CellulesHorsNavmesh <= 0 {
		t.Error("aucune cellule effacee hors maillage")
	}
	if bSans.CellulesHorsNavmesh != 0 {
		t.Errorf("CellulesHorsNavmesh = %d sans maillage, attendu 0", bSans.CellulesHorsNavmesh)
	}
	// Mesure du 2026-11-13 : 56,47 pour cent de la matiere finale (facades et toits qui bordent
	// la rue, hors du maillage elargi de 1,5 m), a deux points pres.
	const partMesuree, tolerance = 0.5647, 0.02
	if math.Abs(part-partMesuree) > tolerance {
		t.Errorf("part rognee = %.4f, attendu %.4f a %.2f pres", part, partMesuree, tolerance)
	}
}

func TestPlanDesMursBazaar(t *testing.T) {
	maillage, _, err := ChargeMaillageNatif(context.Background(), racineDuJeu(t), "ctf_bazaar")
	if err != nil {
		t.Fatal(err)
	}
	plan := PlanDesMurs(maillage)
	t.Logf("plan de Bazaar : %d aretes de bord pour %d sommets et %d faces", len(plan.Aretes), plan.Sommets, plan.Faces)
	if len(plan.Aretes) != 1174 {
		t.Errorf("%d aretes de bord, attendu 1174 (mesure du 2026-11-13)", len(plan.Aretes))
	}
	if plan.Sommets != 2312 || plan.Faces != 670 {
		t.Errorf("taille = %d sommets, %d faces ; attendu 2312 et 670", plan.Sommets, plan.Faces)
	}
	const marge = 1e-9
	for i, a := range plan.Aretes {
		for _, p := range [][2]float64{{a[0], a[1]}, {a[2], a[3]}} {
			if p[0] < maillage.Min.X-marge || p[0] > maillage.Max.X+marge || p[1] < maillage.Min.Y-marge || p[1] > maillage.Max.Y+marge {
				t.Fatalf("arete %d hors de l'emprise du maillage : %v", i, a)
			}
		}
	}
}
