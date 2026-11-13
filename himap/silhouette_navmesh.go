package himap

// silhouette_navmesh.go — LE MODE SILHOUETTE DU ROGNAGE, ET LA RASTERISATION DU MAILLAGE.
//
// Le rognage « faces » garde la matiere a moins de `Marge` des faces du maillage. Il efface donc
// aussi ce qui se dresse AU MILIEU de l'arene sans faire partie du sol marchable : un rocher, un
// pilier, un bassin sont des trous du maillage, et le rognage les retire alors qu'ils sont dans
// la forme de la carte.
//
// Le mode silhouette garde en plus tout ce qu'on ne peut pas atteindre depuis le bord de la
// grille en marchant hors du masque : un trou ENTIEREMENT ENTOURE de maillage est dedans. Un
// vide ouvert sur l'exterieur (une baie, un canyon) reste dehors.

import (
	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// ModeNavmesh est la forme du masque de rognage.
type ModeNavmesh string

const (
	// ModeFaces : le masque est l'ensemble des faces gardees, dilate de `Marge`. Defaut.
	ModeFaces ModeNavmesh = "faces"
	// ModeSilhouette : le masque de ModeFaces, dont on remplit les trous interieurs.
	ModeSilhouette ModeNavmesh = "silhouette"
)

// valide rend le mode a appliquer et dit si celui qui est demande etait connu. Le mode vide est
// ModeFaces ; un mode inconnu est ramene a ModeFaces et signale par `false`.
func (m ModeNavmesh) valide() (ModeNavmesh, bool) {
	switch m {
	case "", ModeFaces:
		return ModeFaces, true
	case ModeSilhouette:
		return ModeSilhouette, true
	}
	return ModeFaces, false
}

// couvreTriangles rend, sur la grille de `nx` x `ny` cellules de cote `cell` dont le coin
// d'indices (0, 0) est en `min`, les cellules dont le centre tombe dans l'un des triangles
// (XY, altitude ignoree). Indice d'une cellule : j*nx + i.
func couvreTriangles(tris [][3]hinavmesh.Point, min [2]float64, cell float64, nx, ny int) []bool {
	out := make([]bool, nx*ny)
	for _, t := range tris {
		a := [3]float64{t[0].X, t[0].Y, t[0].Z}
		b := [3]float64{t[1].X, t[1].Y, t[1].Z}
		c := [3]float64{t[2].X, t[2].Y, t[2].Z}
		x0, x1 := minMax3(a[0], b[0], c[0])
		y0, y1 := minMax3(a[1], b[1], c[1])
		i0 := borne(int((x0-min[0])/cell), nx-1)
		i1 := borne(int((x1-min[0])/cell), nx-1)
		j0 := borne(int((y0-min[1])/cell), ny-1)
		j1 := borne(int((y1-min[1])/cell), ny-1)
		det := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
		for j := j0; j <= j1; j++ {
			y := min[1] + (float64(j)+0.5)*cell
			for i := i0; i <= i1; i++ {
				if _, dedans := altitudeAuPoint(a, b, c, det, min[0]+(float64(i)+0.5)*cell, y); dedans {
					out[j*nx+i] = true
				}
			}
		}
	}
	return out
}

// minMax3 rend le plus petit et le plus grand de trois nombres.
func minMax3(a, b, c float64) (float64, float64) {
	return min(a, b, c), max(a, b, c)
}

// remplitTrous rend le masque `masque` (grille de `nx` x `ny`) auquel on ajoute toutes les
// cellules vides que l'on n'atteint pas depuis le bord de la grille en marchant sur des cellules
// vides (connexite a 4). Parcours en largeur sur une file : aucune recursion, une passe.
func remplitTrous(masque []bool, nx, ny int) []bool {
	if nx <= 0 || ny <= 0 {
		return append([]bool(nil), masque...)
	}
	atteint := make([]bool, len(masque))
	file := make([]int32, 0, 2*(nx+ny))
	pousse := func(i, j int) {
		k := j*nx + i
		if !masque[k] && !atteint[k] {
			atteint[k] = true
			file = append(file, int32(k))
		}
	}
	for i := 0; i < nx; i++ {
		pousse(i, 0)
		pousse(i, ny-1)
	}
	for j := 0; j < ny; j++ {
		pousse(0, j)
		pousse(nx-1, j)
	}
	for tete := 0; tete < len(file); tete++ {
		k := int(file[tete])
		i, j := k%nx, k/nx
		if i > 0 {
			pousse(i-1, j)
		}
		if i < nx-1 {
			pousse(i+1, j)
		}
		if j > 0 {
			pousse(i, j-1)
		}
		if j < ny-1 {
			pousse(i, j+1)
		}
	}
	out := make([]bool, len(masque))
	for k := range masque {
		out[k] = masque[k] || !atteint[k]
	}
	return out
}
