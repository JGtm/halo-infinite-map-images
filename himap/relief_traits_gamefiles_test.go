//go:build gamefiles

package himap

// relief_traits_gamefiles_test.go — les traits du relief de Recharge (sgh_blueprint) et de Behemoth
// (va_behemoth), cuits depuis l'installation du jeu. Recharge epingle les ordres de grandeur de la
// conception (§4.2, definition b) ; Behemoth, la plus lourde des cartes temoins, mesure le temps.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

// ancresBehemoth : positions monde des objectifs de Behemoth (catalogue de LevelUp), doublons retires.
var ancresBehemoth = [][3]float64{
	{-123.80000305175781, 53.70000076293945, 4.1771559715271}, {-133.8000030517578, 53.70000076293945, 8.214308738708496},
	{-102.19999694824219, 53.70000076293945, 10.199999809265137}, {-123.79999542236328, 63.30563735961914, 5.023823261260986},
	{-123.80000305175781, 53.699989318847656, 8.504140853881836}, {-99.23342895507812, 53.710838317871094, 10.210780143737793},
	{-143.31520080566406, 80.53153228759766, 9.116107940673828}, {-141.3000030517578, 41.099998474121094, 9.074999809265137},
	{-105.66368865966797, 81.36105346679688, 9.27299976348877}, {-123.80000305175781, 53.70000076293945, 8},
	{-123.80000305175781, 32.42659378051758, 9.741162300109863}, {-123.79998016357422, 74.9715805053711, 9.741087913513184},
	{-123.80000305175781, 74.97156524658203, 9.744369506835938}, {-123.79975128173828, 53.70090866088867, 8.094696998596191},
	{-123.80000305175781, 32.42659378051758, 9.74116325378418}, {-138.1692352294922, 67.4141845703125, 9.021472930908203},
	{-107.91990661621094, 44.721839904785156, 9.585728645324707}, {-138.1811981201172, 39.922691345214844, 9.053645133972168},
	{-107.92469024658203, 62.83808135986328, 9.585728645324707}, {-123.80000305175781, 53.69999694824219, 8.22314167022705},
	{-123.80000305175781, 53.70000076293945, 8.097224235534668}, {-123.80000305175781, 66.81233978271484, 8.941702842712402},
	{-123.80000305175781, 53.698665618896484, 8.108503341674805}, {-123.80715942382812, 40.66581726074219, 8.939273834228516},
}

// calageBehemothPublie : le calage du fond publie de Behemoth (va_behemoth.json, 2026-11-14).
var calageBehemothPublie = CalageFond{
	OrigineX: -163.57795368830364, OrigineY: 97.7515103975932, MetresParPixel: 0.049644819895426434, Largeur: 1601, Hauteur: 1776,
}

// reliefCuit cuit une carte native comme le fait TestReliefRecharge (rognage au maillage, silhouette)
// et rend son relief dans le calage publie `cal`.
func reliefCuit(t *testing.T, module string, ancres [][3]float64, cal CalageFond, comble bool) *Relief {
	t.Helper()
	racine := racineDuJeu(t)
	ctx := context.Background()
	maillage, _, err := ChargeMaillageNatif(ctx, racine, module)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := CuitCarteNative(ctx, OptionsCuisson{
		RacineDeploy: racine, CheminModule: moduleDuJeu(t, "pc", module), Ancres: ancres,
		Echelle: cal.MetresParPixel, EcreteToits: !comble, PlafondArene: plafondDe(comble), SansEau: !comble,
		CombleTrous: comble, CombleZonesEntieres: comble,
		Navmesh: OptionsNavmesh{Maillage: maillage, Rogne: true, Mode: ModeSilhouette},
	})
	if err != nil {
		t.Fatal(err)
	}
	matiere := func(x, y float64) bool {
		i, j := int(math.Floor((x-r.Min[0])/r.Cell)), int(math.Floor((y-r.Min[1])/r.Cell))
		_, ok := r.Altitude(i, j)
		return ok
	}
	retenues := ComposantesRetenues(maillage, ancres, matiere)
	rel, err := CalculeRelief(ctx, r, maillage, retenues.Garde, cal)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

// plafondDe : le plafond d'arene des reglages de Recharge (4), aucun pour Behemoth.
func plafondDe(comble bool) float64 {
	if comble {
		return 0
	}
	return 4
}

// mesureTraits lance CalculeTraits, le chronometre et rend le bilan, le poids JSON des traits et ce
// poids apres gzip (niveau 5, celui de l'API).
func mesureTraits(t *testing.T, rel *Relief) (BilanTraits, time.Duration, int, int) {
	t.Helper()
	debut := time.Now()
	b, err := rel.CalculeTraits(context.Background())
	duree := time.Since(debut)
	if err != nil {
		t.Fatal(err)
	}
	brut, err := json.Marshal(rel.Donnees.Lines)
	if err != nil {
		t.Fatal(err)
	}
	var z bytes.Buffer
	w, _ := gzip.NewWriterLevel(&z, 5)
	if _, err := w.Write(brut); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("traits en %v ; JSON %d o, gzip %d o", duree, len(brut), z.Len())
	t.Logf("ruptures %+v", b.Ruptures)
	t.Logf("courbes  %+v", b.Courbes)
	t.Logf("lisere   %+v", b.Liseres)
	t.Logf("jonctions %d, gardees %d", b.Jonctions, b.JonctionsGardees)
	return b, duree, len(brut), z.Len()
}

// Recharge : la longueur brute des ruptures et le nombre de jonctions de la conception (§4.2, definition
// b : 1 172 u et 919 jonctions), les courbes et le poids des traits, et le determinisme sur une vraie
// carte. Mesure du 2026-11-18 : 1 181,8 u, 920 jonctions, 44 courbes, 44,0 Ko et 12,3 Ko apres gzip.
// Recharge est faite de sols plats : ses rares courbes sont ses rampes. Sans l'ecart aux ruptures
// (RayonSansCourbePresDUneRupture), le lissage doublait chaque mur et donnait 1 584 courbes et 107,8 Ko ;
// la borne haute des courbes l'interdit. La cuisson de ce test omet les zones nommees que la conception
// avait (part du relief tiree du maillage 13,6 % contre 13,8 %, TestReliefRecharge) : 5 % sur la longueur
// et les jonctions en couvrent l'ecart sans laisser passer la definition a (1 322 u, 1 335 jonctions) ;
// 25 % sur un compte de 44 courbes, 15 % sur les poids.
func TestTraitsRecharge(t *testing.T) {
	rel := reliefCuit(t, "sgh_blueprint", ancresRecharge, calageRechargePublie, false)
	b, _, brut, gz := mesureTraits(t, rel)
	attendsProche(t, "longueur brute des ruptures (u)", b.Ruptures.LongueurBrute, 1172, 0.05)
	attendsProche(t, "jonctions", float64(b.Jonctions), 919, 0.05)
	attendsProche(t, "courbes", float64(b.Courbes.Polylignes), 44, 0.25)
	attendsProche(t, "JSON des traits (Ko)", float64(brut)/1024, 44.0, 0.15)
	attendsProche(t, "JSON des traits apres gzip (Ko)", float64(gz)/1024, 12.3, 0.15)
	l := rel.Donnees.Lines
	if len(l.Ruptures) == 0 || len(l.Courbes) == 0 || len(l.Liseres) == 0 {
		t.Fatalf("famille vide : %d ruptures, %d courbes, %d anneaux", len(l.Ruptures), len(l.Courbes), len(l.Liseres))
	}
	avant, _ := json.Marshal(l)
	if _, err := rel.CalculeTraits(context.Background()); err != nil {
		t.Fatal(err)
	}
	if apres, _ := json.Marshal(rel.Donnees.Lines); !bytes.Equal(avant, apres) {
		t.Error("deux calculs des traits de Recharge, deux sorties")
	}
}

// Behemoth, la plus lourde des quatre cartes temoins (1601 x 1776 px, 2,84 millions de pixels) : le
// calcul des traits tient sous quelques secondes (mesure du 2026-11-18 : 0,22 s). Ses ordres de grandeur
// sont ceux de la conception (§4.2, definition b : 1 275 u, 838 jonctions) ; mesure : 1 303 u, 890
// jonctions, 683 courbes (2 801 sans l'ecart aux ruptures), 75,4 Ko et 24,2 Ko apres gzip. La cuisson de
// ce test n'a ni les zones nommees ni les reglages par defaut de la chaine de LevelUp (part tiree du
// maillage 44,9 % contre 46,3 %) : 10 % en couvrent l'ecart sur les ruptures, loin de la definition a
// (2 151 u, 1 963 jonctions) ; 15 % sur les courbes et les poids.
func TestTraitsBehemothDuree(t *testing.T) {
	rel := reliefCuit(t, "va_behemoth", ancresBehemoth, calageBehemothPublie, true)
	b, duree, brut, gz := mesureTraits(t, rel)
	if duree > dureeMaxTraits {
		t.Errorf("traits de Behemoth en %v, attendu moins de %v", duree, dureeMaxTraits)
	}
	attendsProche(t, "longueur brute des ruptures (u)", b.Ruptures.LongueurBrute, 1275, 0.10)
	attendsProche(t, "jonctions", float64(b.Jonctions), 838, 0.10)
	attendsProche(t, "courbes", float64(b.Courbes.Polylignes), 683, 0.15)
	attendsProche(t, "JSON des traits (Ko)", float64(brut)/1024, 75.4, 0.15)
	attendsProche(t, "JSON des traits apres gzip (Ko)", float64(gz)/1024, 24.2, 0.15)
}

// dureeMaxTraits : borne du calcul des traits d'une carte, large devant la mesure (0,22 s sur Behemoth)
// pour tenir sur un poste plus lent.
const dureeMaxTraits = 5 * time.Second

// attendsProche verifie qu'une mesure est a une part relative `tol` de la valeur attendue.
func attendsProche(t *testing.T, nom string, got, attendu, tol float64) {
	t.Helper()
	if math.Abs(got-attendu) > tol*attendu {
		t.Errorf("%s = %.1f, attendu %.1f a %.0f %% pres", nom, got, attendu, 100*tol)
	}
}
