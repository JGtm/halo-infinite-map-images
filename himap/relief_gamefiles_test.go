//go:build gamefiles

package himap

// relief_gamefiles_test.go — le relief de Recharge (sgh_blueprint), cuit depuis l'installation du jeu
// et projete dans le calage du fond PUBLIE. Mesures du 2026-11-18.

import (
	"context"
	"math"
	"testing"
)

// ancresRecharge : positions monde des objectifs de Recharge, les deux entrees du catalogue de
// LevelUp (Recharge et Recharge - Ranked) fondues, doublons retires. Elles sont a la precision du
// catalogue : arrondies, elles decaleraient l'emprise du rendu de quelques millimetres, et le cadre
// publie ne tomberait plus sur un decalage entier de la grille cuite.
var ancresRecharge = [][3]float64{
	{8.328492164611816, -15.628952980041504, 3.2305097579956055}, {20.47776222229004, 8.386775970458984, 2.1925761699676514},
	{2.9592506885528564, -5.223813056945801, 2.221837282180786}, {27.23837661743164, -2.700000286102295, 2.1999998092651367},
	{0.38979101181030273, 4.63340950012207, 2.2151882648468018}, {26.700000762939453, -6.800000190734863, 2.200000047683716},
	{-1.648560643196106, 5.720921516418457, 2.8808467388153076}, {-1.6552530527114868, 5.476814270019531, 2.885593891143799},
	{28.095861434936523, 2.0009429454803467, 2.213494300842285}, {14.83034610748291, -2.4759585857391357, 0.02981051802635193},
	{14.861748695373535, -2.808267116546631, -0.016464829444885254}, {22.7742862701416, 4.3128886222839355, 2.140660047531128},
	{0.3246755599975586, 4.26894998550415, 2.1728320121765137}, {17.600000381469727, 9.200000762939453, 0},
	{8.472000122070312, -5.2515997886657715, 2.200000047683716}, {14.856266021728516, -10, 2.200000047683716},
	{27.237489700317383, -1.287200927734375, 2.20139741897583}, {1.7739158868789673, 3.295926570892334, 2.2022945880889893},
	{14.867605209350586, -7.60009765625, 2.1911959648132324}, {22.892202377319336, 9.04111385345459, 2.2097108364105225},
	{2.830946445465088, 2.893719434738159, 4.407018661499023}, {14.881858825683594, -10.199999809265137, 2.190457344055176},
	{23.213001251220703, -2.98799991607666, 3.2249999046325684}, {14.812975883483887, -3.408259153366089, -0.6164647340774536},
	{14.875007629394531, -12.227070808410645, 2.0728321075439453},
}

// calageRechargePublie : le calage du fond publie de Recharge (sgh_blueprint.json, 2026-11-13). La
// recuisson d'aujourd'hui sort un cadre de 1605 x 1736 px : 35 lignes de moins, en haut.
var calageRechargePublie = CalageFond{
	OrigineX: -10.011253052711481, OrigineY: 28.708047019958506, MetresParPixel: 0.029, Largeur: 1605, Hauteur: 1771,
}

// Recharge : image et zone de la taille du cadre publie (1605 x 1771), cadre publie pose sur un
// decalage ENTIER de la grille cuite (la cuisson de ce test n'est pas rognee a la matiere : le cadre
// publie en est une fenetre), part du relief tiree du maillage proche de 13,8 % (conception §4.2,
// definition b ; la definition a donne 4,3 %). Cette cuisson omet les zones nommees (callouts) et
// leur aplat de sol suppose, que la brique ne porte pas : le dessus du rendu en differe un peu. Ecart
// mesure le 2026-11-18 : 13,6 % ; la tolerance d'un point en couvre le bruit sans laisser passer la
// definition a.
func TestReliefRecharge(t *testing.T) {
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
	cal := calageRechargePublie
	matiere := func(x, y float64) bool {
		i, j := int(math.Floor((x-r.Min[0])/r.Cell)), int(math.Floor((y-r.Min[1])/r.Cell))
		_, ok := r.Altitude(i, j)
		return ok
	}
	retenues := ComposantesRetenues(maillage, ancresRecharge, matiere)
	rel, err := CalculeRelief(ctx, r, maillage, retenues.Garde, cal)
	if err != nil {
		t.Fatal(err)
	}
	b := rel.Bilan
	t.Logf("cuite %d x %d ; cadre publie %d x %d ; decalage (%.3f, %.3f) entier %v ; hors grille %d ; matiere perdue %d/%d",
		r.NX, r.NY, cal.Largeur, cal.Hauteur, b.DecalageX, b.DecalageY, b.DecalageEntier, b.HorsGrille, b.MatierePerdue, b.MatiereCuite)
	t.Logf("connus %d, zone %d, sol du maillage %d (part %.4f), miettes %d, hauteur %+v", b.Connus, b.Zone, b.SolMaillage,
		b.PartMaillage, b.Miettes, rel.Donnees.Height)

	if got := rel.Image.Bounds(); got.Dx() != cal.Largeur || got.Dy() != cal.Hauteur {
		t.Errorf("image %d x %d, attendu %d x %d (le calage publie)", got.Dx(), got.Dy(), cal.Largeur, cal.Hauteur)
	}
	if len(rel.Zone) != cal.Largeur*cal.Hauteur {
		t.Errorf("zone de %d pixels, attendu %d", len(rel.Zone), cal.Largeur*cal.Hauteur)
	}
	const partMesuree, tolerance = 0.138, 0.01
	if math.Abs(b.PartMaillage-partMesuree) > tolerance {
		t.Errorf("part tiree du maillage = %.4f, attendu %.3f a %.2f pres", b.PartMaillage, partMesuree, tolerance)
	}
	if !b.DecalageEntier || b.HorsGrille != 0 || b.MatierePerdue != 0 {
		t.Errorf("decalage (%.3f, %.3f) entier %v, hors grille %d, matiere perdue %d ; attendu une fenetre entiere de la grille",
			b.DecalageX, b.DecalageY, b.DecalageEntier, b.HorsGrille, b.MatierePerdue)
	}
	if h := rel.Donnees.Height; h.Z0 != -7 || math.Abs(h.Haut-11.38) > 0.1 {
		t.Errorf("hauteurs %+v, attendu z0 -7 et haut 11,38 (sidecar de la conception)", h)
	}
	if b.SansMaillage || len(rel.Donnees.Degradations) != 0 {
		t.Errorf("sans maillage %v, degradations %v", b.SansMaillage, rel.Donnees.Degradations)
	}
}
