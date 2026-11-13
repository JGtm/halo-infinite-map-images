package himap

// composantes_navmesh.go — QUELLES COMPOSANTES DU MAILLAGE FORMENT L'ARENE.
//
// Un maillage de navigation porte parfois, en plus de l'arene, des dalles qui n'en font pas
// partie : sur quinze cartes Forge la plus grande composante est le sol du canevas, pas
// l'arene. Garder « la plus grande » est donc faux ; garder « celles qui portent une ancre » est
// faux aussi, car un ilot jouable sans objectif (un socle d'arme isole) n'a pas d'ancre.
//
// LA REGLE (mesuree sur les 111 cartes le 2026-11-13). Une composante est gardee si :
//
//	(a) elle porte au moins une ancre d'objectif DONT l'appui est vertical : la position XY de
//	    l'ancre tombe dans l'une de ses faces ET son altitude est a moins de AppuiVerticalAncre
//	    de cette face (une ancre loin au-dessus d'une dalle ne la designe pas) ; OU
//	(b) elle fait au moins AireMinimaleIlot ET au moins PartMinimaleSurMatiere de son aire tombe
//	    sur la matiere du rendu (le rendu a dessine cet ilot : c'est du terrain, pas un vide).
//
// Repli : si aucune composante ne passe (aucune ancre, aucune matiere), on garde la plus grande
// par aire, pour ne jamais rendre un maillage vide a un maillage qui ne l'est pas.
//
// Le critere (b) n'existe qu'une fois le rendu projete : la selection faite AVANT la reference
// (sans matiere) ne connait que (a) et le repli, celle du rognage connait les deux.

import (
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

const (
	// AppuiVerticalAncre : ecart maximal, en metres, entre l'altitude d'une ancre et celle de la
	// face qu'elle surplombe pour que cette face la porte.
	AppuiVerticalAncre = 2.0
	// AireMinimaleIlot : aire au sol minimale, en m2, d'une composante sans ancre pour etre
	// gardee sur la foi de la matiere. En dessous, c'est un debris du maillage.
	AireMinimaleIlot = 4.0
	// PartMinimaleSurMatiere : part de l'aire d'une composante qui doit tomber sur la matiere
	// du rendu pour qu'elle soit gardee par le critere (b).
	PartMinimaleSurMatiere = 0.90
	// pasEchantillonMatiere : pas, en metres, de l'echantillonnage de l'aire d'une composante
	// pour la part sur matiere (une cellule du fond fait environ 0,09 m).
	pasEchantillonMatiere = 0.25
	// subdivisionMaxEchantillon borne le nombre de subdivisions par cote d'un triangle.
	subdivisionMaxEchantillon = 64
)

// Retenues est le resultat de ComposantesRetenues.
type Retenues struct {
	// Garde dit, par indice de face de Maillage.Faces, si la face appartient a une composante
	// retenue. Vide pour un maillage vide.
	Garde []bool
	// Gardees et Ecartees : nombre de composantes retenues et ecartees.
	Gardees, Ecartees int
	// AireGardee et AireEcartee : aire au sol cumulee des unes et des autres, en m2.
	AireGardee, AireEcartee float64
	// ParAncre : composantes gardees par le critere (a). ParMatiere : gardees par le seul
	// critere (b).
	ParAncre, ParMatiere int
	// Repli est vrai quand aucune composante ne passait et que la plus grande a ete gardee.
	Repli bool
}

// ComposantesRetenues choisit les composantes du maillage qui forment l'arene, selon la regle
// du fichier. `matiere` dit si le rendu porte de la matiere au point (x, y) monde ; nil
// desactive le critere (b). Un maillage nil ou vide rend un resultat vide.
func ComposantesRetenues(m *hinavmesh.Maillage, ancres [][3]float64, matiere func(x, y float64) bool) Retenues {
	comps := hinavmesh.Composantes(m)
	if len(comps) == 0 {
		return Retenues{Garde: []bool{}}
	}
	var res Retenues
	gardee := make([]bool, len(comps))
	for i, c := range comps {
		tris := m.TrianglesDe(c.Garde)
		switch {
		case ancreAppuyee(tris, ancres):
			gardee[i] = true
			res.ParAncre++
		case matiere != nil && c.Aire >= AireMinimaleIlot && partSurMatiere(tris, matiere) >= PartMinimaleSurMatiere:
			gardee[i] = true
			res.ParMatiere++
		}
	}
	if res.ParAncre+res.ParMatiere == 0 {
		gardee[0] = true // les composantes sont triees par aire decroissante
		res.Repli = true
	}
	res.Garde = make([]bool, len(m.Faces))
	for i, c := range comps {
		if !gardee[i] {
			res.Ecartees++
			res.AireEcartee += c.Aire
			continue
		}
		res.Gardees++
		res.AireGardee += c.Aire
		for f, ok := range c.Garde {
			if ok {
				res.Garde[f] = true
			}
		}
	}
	return res
}

// ancreAppuyee dit si au moins une ancre tombe, en XY, dans un des triangles ET a moins de
// AppuiVerticalAncre de leur surface a cet endroit.
func ancreAppuyee(tris [][3]hinavmesh.Point, ancres [][3]float64) bool {
	for _, t := range tris {
		a := [3]float64{t[0].X, t[0].Y, t[0].Z}
		b := [3]float64{t[1].X, t[1].Y, t[1].Z}
		c := [3]float64{t[2].X, t[2].Y, t[2].Z}
		det := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
		for _, p := range ancres {
			z, dedans := altitudeAuPoint(a, b, c, det, p[0], p[1])
			if dedans && math.Abs(p[2]-z) <= AppuiVerticalAncre {
				return true
			}
		}
	}
	return false
}

// partSurMatiere rend la part, entre 0 et 1, de l'aire au sol des triangles dont l'echantillon
// tombe sur de la matiere. Chaque triangle est decoupe en k*k sous-triangles (k suit sa plus
// longue arete, au pas pasEchantillonMatiere) dont on teste le centre, au poids de leur aire.
func partSurMatiere(tris [][3]hinavmesh.Point, matiere func(x, y float64) bool) float64 {
	e := echantillonMatiere{matiere: matiere}
	for _, t := range tris {
		aire := math.Abs((t[1].X-t[0].X)*(t[2].Y-t[0].Y)-(t[2].X-t[0].X)*(t[1].Y-t[0].Y)) / 2
		if aire == 0 {
			continue
		}
		longueur := math.Max(math.Hypot(t[1].X-t[0].X, t[1].Y-t[0].Y),
			math.Max(math.Hypot(t[2].X-t[1].X, t[2].Y-t[1].Y), math.Hypot(t[0].X-t[2].X, t[0].Y-t[2].Y)))
		k := max(1, min(int(math.Ceil(longueur/pasEchantillonMatiere)), subdivisionMaxEchantillon))
		poids := aire / float64(k*k)
		fk := float64(k)
		for r := 0; r < k; r++ {
			for c := 0; c+r < k; c++ {
				e.pese(t, (float64(c)+1.0/3)/fk, (float64(r)+1.0/3)/fk, poids)
				if c+r < k-1 {
					e.pese(t, (float64(c)+2.0/3)/fk, (float64(r)+2.0/3)/fk, poids)
				}
			}
		}
	}
	if e.total == 0 {
		return 0
	}
	return e.sur / e.total
}

// echantillonMatiere cumule l'aire echantillonnee et celle qui tombe sur de la matiere.
type echantillonMatiere struct {
	matiere    func(x, y float64) bool
	total, sur float64
}

// pese teste le point du triangle `t` de coordonnees barycentriques (u, v) le long de ses deux
// aretes issues du premier sommet, au poids `poids`.
func (e *echantillonMatiere) pese(t [3]hinavmesh.Point, u, v, poids float64) {
	x := t[0].X + u*(t[1].X-t[0].X) + v*(t[2].X-t[0].X)
	y := t[0].Y + u*(t[1].Y-t[0].Y) + v*(t[2].Y-t[0].Y)
	if e.matiere(x, y) {
		e.sur += poids
	}
	e.total += poids
}
