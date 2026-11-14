package himap

// navmesh_commun.go — LE MAILLAGE DE NAVIGATION DANS LES DEUX CHAINES DE CUISSON.
//
// Une carte Forge le lit dans son `navmesh.blob` (hinavmesh.Decode), une carte native dans la
// ressource de maillage de son module `any` (ChargeMaillageNatif) ; une fois decode c'est le
// meme objet, et il sert aux deux chaines de la meme facon. Ce fichier est le SEUL endroit qui
// l'applique a un rendu : `CuitCarteForge` et `CuitCarteNative` passent par `appliqueNavmesh`
// (garde-rail : navmesh_commun_test.go).

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// OptionsNavmesh decrit ce qu'une cuisson fait du maillage de navigation de sa carte.
// La valeur zero ne fait rien : sans `Maillage`, aucun des autres champs ne s'applique, sauf
// `Tolerance` qui n'a d'effet que si une reference de maillage a ete armee.
type OptionsNavmesh struct {
	// Maillage sert de SURFACE DE REFERENCE a la place de l'interpolation des ancres. Le rendu
	// reste celui de la geometrie ordinaire : on ne change que ce a quoi on compare les
	// surfaces. Nil = pas de maillage, la reference reste celle des ancres.
	Maillage *hinavmesh.Maillage
	// Rogne efface la matiere hors du maillage, dilate de `Marge`.
	Rogne bool
	// Mode : forme du masque de rognage. Vide ou ModeFaces : les faces gardees, dilatees de
	// `Marge`. ModeSilhouette : idem, trous interieurs remplis (rochers, piliers et eau au milieu
	// de l arene restent dessines). Un mode inconnu est journalise et traite comme ModeFaces.
	Mode ModeNavmesh
	// Marge : dilatation du masque de rognage, en metres. ZERO = `MargeNavmesh` (0,5 m ; c etait
	// 3 m sur les DEUX chaines jusqu au 2026-11-13).
	//
	// La marge garde les MURS qui bordent le sol : le maillage se retire des parois, et rogner
	// au ras rendrait une carte sans ses cloisons. Mais elle garde aussi ce qui borde le sol
	// sans en etre (un rocher contre l'arene, une falaise) : la resserrer est le levier qui
	// distingue ces masses du terrain.
	Marge float64
	// Tolerance vide les cellules dont la surface retenue s'ecarte de plus de N metres du sol
	// donne par le maillage. Zero = ne rien vider.
	Tolerance float64
	// NiveauHaut : prendre le niveau le PLUS HAUT du maillage comme reference la ou deux
	// niveaux se superposent, pour que les etages et les passerelles survivent a la
	// substitution. Sert a l armement de la reference (ArmeReferenceDepuisNavmesh).
	NiveauHaut bool

	// ancres : positions monde des objectifs, posees par la chaine de cuisson avant
	// l etape de reference. Elles servent a choisir les composantes du maillage retenues.
	ancres [][3]float64
}

// BilanNavmesh est ce qu'une cuisson a fait du maillage de navigation : forme et marge du
// rognage, et le sort des composantes du maillage (voir ComposantesRetenues).
type BilanNavmesh struct {
	// Mode et Marge : ce que le rognage a appliqué. Vides tant qu aucun rognage n a eu lieu.
	Mode  ModeNavmesh
	Marge float64
	// Gardees et Ecartees : composantes du maillage retenues et ecartees ; AireGardee et
	// AireEcartee : leur aire au sol cumulee, en m2. ParAncre et ParMatiere : par quel critere
	// les gardees l ont ete. Repli : aucune ne passait, la plus grande a ete gardee.
	Gardees, Ecartees       int
	AireGardee, AireEcartee float64
	ParAncre, ParMatiere    int
	Repli                   bool
	// CellulesFinales et EauRetiree : cellules de matiere (surface relevee ou sol suppose), puis
	// cellules d'eau visibles, que l'etape finale a retirees hors de la silhouette. Nuls tant
	// qu'aucun rognage n'a pose de silhouette.
	CellulesFinales, EauRetiree int
}

// note recopie le resultat d une selection de composantes dans le bilan.
func (n *BilanNavmesh) note(res Retenues) {
	n.Gardees, n.Ecartees = res.Gardees, res.Ecartees
	n.AireGardee, n.AireEcartee = res.AireGardee, res.AireEcartee
	n.ParAncre, n.ParMatiere, n.Repli = res.ParAncre, res.ParMatiere, res.Repli
}

// etapeNavmesh designe le moment de la cuisson ou le maillage s'applique.
type etapeNavmesh int

const (
	// etapeReference : AVANT la projection de la geometrie, car la voie de reference remplit
	// ses tampons pendant la projection. Remplace la reference des ancres.
	etapeReference etapeNavmesh = iota
	// etapeRognage : APRES la substitution, sur l'image qu'elle a rendue.
	etapeRognage
	// etapePlan : AVANT le rognage aux zones nommees, qui doit connaitre la silhouette du plan :
	// pose le masque du rognage (voir silhouetteDuPlan). Le rognage au maillage le recalcule plus tard.
	etapePlan
	// etapeFinale : EN TOUT DERNIER, apres l'eau et tout autre calque peint : recoupe le rendu a
	// la silhouette du rognage (voir recoupeALaSilhouette).
	etapeFinale
)

// appliqueNavmesh applique a un rendu la part de `o` qui revient a `etape`. Sans maillage
// (`o.Maillage == nil`) l'etape de reference ne fait rien, et celle de rognage ne rogne
// qu'a la tolerance.
//
// Les composantes du maillage sont choisies DEUX fois : a la reference (avant la projection, donc
// sans matiere : ancres et repli seulement), puis a l'etape de rognage (avec la matiere du rendu,
// qui sauve les ilots jouables sans objectif), TOUJOURS, meme sans `Rogne` : c'est elle qui pose
// la couverture finale du maillage. Le bilan garde la derniere.
func appliqueNavmesh(ctx context.Context, r *Rendu, b *BilanCuisson, o OptionsNavmesh, etape etapeNavmesh) {
	switch etape {
	case etapeReference:
		if o.Maillage == nil {
			return
		}
		res := ComposantesRetenues(o.Maillage, o.ancres, nil)
		b.Navmesh.note(res)
		couvertes := r.ArmeReferenceDepuisNavmesh(o.Maillage, res.Garde, o.NiveauHaut)
		slog.InfoContext(ctx, "mapfond: reference prise sur le maillage de navigation",
			"carte", b.Module, "cellules", couvertes, "composantes", fmt.Sprintf("%d gardees, %d ecartees", res.Gardees, res.Ecartees))
	case etapePlan:
		silhouetteDuPlan(r, o)
	case etapeFinale:
		recoupeALaSilhouette(ctx, r, b)
	case etapeRognage:
		if o.Maillage != nil {
			rogneAuMaillage(ctx, r, b, o)
		}
		if o.Tolerance > 0 {
			n := r.EffaceLoinDuNavmesh(o.Tolerance)
			slog.InfoContext(ctx, "mapfond: surfaces loin du sol vidées",
				"carte", b.Module, "tolerance", o.Tolerance, "cellules", n)
		}
	}
}

// margeEffective rend la marge de rognage : `Marge`, ou `MargeNavmesh` quand elle est nulle.
func (o OptionsNavmesh) margeEffective() float64 {
	if o.Marge > 0 {
		return o.Marge
	}
	return MargeNavmesh
}

// rogneAuMaillage choisit les composantes avec la matiere du rendu et pose la couverture finale
// du maillage (celle que lira CombleDansLeMaillage), que `Rogne` soit armee ou non. Il n'efface
// la matiere hors du masque, dans le mode et a la marge de `o`, que si `Rogne` l'est.
//
// Un ilot garde par la seule matiere n'a pas de reference (la selection de reference ne
// connaissait que les ancres) : ni substitution ni filtre de tolerance ne le touchent.
func rogneAuMaillage(ctx context.Context, r *Rendu, b *BilanCuisson, o OptionsNavmesh) {
	res := ComposantesRetenues(o.Maillage, o.ancres, r.matiereEn)
	b.Navmesh.note(res)
	r.poseCouvertureNavmesh(o.Maillage, res.Garde)
	if !o.Rogne {
		return
	}
	marge := o.margeEffective()
	mode, connu := o.Mode.valide()
	if !connu {
		slog.WarnContext(ctx, "mapfond: mode de rognage au maillage inconnu, mode faces utilise",
			"carte", b.Module, "mode", string(o.Mode))
	}
	b.Navmesh.Mode, b.Navmesh.Marge = mode, marge
	r.masqueFinal = r.masqueDeRognage(mode, marge)
	n := r.effaceHorsMasque(r.masqueFinal)
	b.CellulesHorsNavmesh = n
	slog.InfoContext(ctx, "mapfond: matiere effacee hors du maillage de navigation",
		"carte", b.Module, "cellules", n, "mode", string(mode), "marge", marge,
		"composantes", fmt.Sprintf("%d gardees (%.0f m2, %d par ancre, %d par matiere, repli %t), %d ecartees (%.0f m2)",
			res.Gardees, res.AireGardee, res.ParAncre, res.ParMatiere, res.Repli, res.Ecartees, res.AireEcartee))
}

// recoupeALaSilhouette est l'etape finale des deux chaines : le rendu est coupe a la silhouette
// du plan, celle que le rognage a posee (composantes gardees, marge resolue, trous interieurs
// remplis en mode silhouette), apres l'eau et tout calque peint depuis. L'eau se pose apres le
// rognage et ne connait pas le masque : sans cette etape elle depasse de la vue plan. Sans
// rognage au maillage (`Rogne` desarme), il n'y a pas de silhouette et rien n'est coupe. Aucune
// selection n'est refaite : c'est le masque du rognage, tel quel.
func recoupeALaSilhouette(ctx context.Context, r *Rendu, b *BilanCuisson) {
	if r.masqueFinal == nil {
		return
	}
	matiere, eau := r.effaceHorsMasqueFinal()
	b.Navmesh.CellulesFinales, b.Navmesh.EauRetiree = matiere, eau
	b.CellulesEau -= eau
	slog.InfoContext(ctx, "mapfond: rendu recoupe a la silhouette du plan",
		"carte", b.Module, "matiere", matiere, "eau", eau)
}

// silhouetteDuPlan pose sur le rendu le masque que le rognage au maillage appliquera : composantes
// choisies avec la matiere du moment, marge resolue, trous interieurs remplis en mode silhouette.
// Le plan est le masque de reference : le rognage aux zones nommees, qui passe avant le rognage
// au maillage sur la chaine native, ne doit jamais effacer une cellule qui y tombe
// (voir Rendu.avecLaSilhouetteDuPlan). Sans maillage ni `Rogne`, il n'y a pas de silhouette et
// rien n'est pose. Rien n'est note au bilan : la selection definitive reste celle du rognage.
func silhouetteDuPlan(r *Rendu, o OptionsNavmesh) {
	if o.Maillage == nil || !o.Rogne {
		return
	}
	res := ComposantesRetenues(o.Maillage, o.ancres, r.matiereEn)
	couverture := couvreTriangles(o.Maillage.TrianglesDe(res.Garde), r.Min, r.Cell, r.NX, r.NY)
	mode, _ := o.Mode.valide()
	r.masqueFinal = r.masqueDeCouverture(couverture, mode, o.margeEffective())
}
