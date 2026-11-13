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
	// Marge : dilatation du masque de rognage, en metres. ZERO = `MargeNavmesh` (3 m).
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
}

// etapeNavmesh designe le moment de la cuisson ou le maillage s'applique.
type etapeNavmesh int

const (
	// etapeReference : AVANT la projection de la geometrie, car la voie de reference remplit
	// ses tampons pendant la projection. Remplace la reference des ancres.
	etapeReference etapeNavmesh = iota
	// etapeRognage : APRES la substitution, sur l'image qu'elle a rendue.
	etapeRognage
)

// appliqueNavmesh applique a un rendu la part de `o` qui revient a `etape`. Sans maillage
// (`o.Maillage == nil`) l'etape de reference ne fait rien, et celle de rognage ne rogne
// qu'a la tolerance.
func appliqueNavmesh(ctx context.Context, r *Rendu, b *BilanCuisson, o OptionsNavmesh, etape etapeNavmesh) {
	switch etape {
	case etapeReference:
		if o.Maillage == nil {
			return
		}
		couvertes := r.ArmeReferenceDepuisNavmesh(o.Maillage, o.NiveauHaut)
		slog.InfoContext(ctx, "mapfond: reference prise sur le maillage de navigation",
			"carte", b.Module, "cellules", couvertes)
	case etapeRognage:
		if o.Rogne && o.Maillage != nil {
			marge := MargeNavmesh
			if o.Marge > 0 {
				marge = o.Marge
			}
			n := r.EffaceHorsNavmesh(marge)
			b.CellulesHorsNavmesh = n
			slog.InfoContext(ctx, "mapfond: matiere effacee hors du maillage de navigation",
				"carte", b.Module, "cellules", n, "marge", marge)
		}
		if o.Tolerance > 0 {
			n := r.EffaceLoinDuNavmesh(o.Tolerance)
			slog.InfoContext(ctx, "mapfond: surfaces loin du sol vidées",
				"carte", b.Module, "tolerance", o.Tolerance, "cellules", n)
		}
	}
}
