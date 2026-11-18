package himap

// relief_zone.go — LA ZONE GARDEE DU RELIEF.
//
// La zone est ce que le fond Relief teinte et ce que la coupe par etage garde : un pixel y est si sa
// hauteur est connue ET s'il est dans le maillage de navigation, traite ainsi (conception §4.3) :
//
//  1. les sols gardes sont elargis de MargeNavmesh (les murs et les rebords qui bordent le sol) ;
//  2. les passages plus etroits que deux fois FermetureZone sont refermes (dilatation puis erosion) ;
//  3. les creux enclos sont combles (remplitTrous : ce que le bord de la grille n'atteint pas, en
//     marchant hors de la zone, est dedans — un rocher ou un pilier au milieu de l'arene, pas une
//     baie ouverte) ;
//  4. les miettes (composante de moins de AireMinAnneau) s'eteignent.
//
// Sans maillage (D-7), la zone est l'emprise de la matiere : tous les pixels de hauteur connue.
// Les faces gardees sont celles de ComposantesRetenues avec la matiere du fond publie, comme le plan
// des murs publie : le relief et le plan tiennent alors aux memes composantes.

import "math"

// zoneGardee rend le masque de la zone (hauteur connue ET dans la zone), miettes eteintes, et pose
// Bilan.Miettes. Les trois premiers temps ne servent qu'avec un maillage present.
func (rel *Relief) zoneGardee(sols *solsMaillage) []bool {
	cal := rel.calage
	base := make([]bool, len(rel.connu))
	if sols.present {
		dedans := zoneDuMaillage(sols.couvre, cal)
		for k, ok := range rel.connu {
			base[k] = ok && dedans[k]
		}
	} else {
		copy(base, rel.connu)
		rel.Donnees.Degradations = append(rel.Donnees.Degradations, DegradationSansMaillage)
	}
	zone, miettes := eteintLesMiettes(base, cal)
	rel.Bilan.Miettes = miettes
	return zone
}

// zoneDuMaillage rend la zone que dessine la couverture du maillage : elargie de MargeNavmesh,
// passages etroits refermes, creux enclos combles.
func zoneDuMaillage(couvre []bool, cal CalageFond) []bool {
	w, h, mpp := cal.Largeur, cal.Hauteur, cal.MetresParPixel
	marge := max(1, int(math.Round(MargeNavmesh/mpp)))
	ferme := max(1, int(math.Round(FermetureZone/mpp)))
	elargie := dilate(couvre, w, h, marge)
	fermee := erodeMasque(dilate(elargie, w, h, ferme), w, h, ferme)
	return remplitTrous(fermee, w, h)
}

// erodeMasque retrecit un masque de k cellules : le complement dilate, complemente. Hors de la grille,
// on ne retrecit pas (la dilatation ne sort pas de la grille).
func erodeMasque(m []bool, nx, ny, k int) []bool {
	inverse := make([]bool, len(m))
	for i, v := range m {
		inverse[i] = !v
	}
	out := dilate(inverse, nx, ny, k)
	for i := range out {
		out[i] = !out[i]
	}
	return out
}

// eteintLesMiettes rend le masque sans ses composantes (4-connexite) de moins de AireMinAnneau, et
// leur nombre de pixels. Une composante est un parcours en largeur sur une file : aucune recursion.
func eteintLesMiettes(masque []bool, cal CalageFond) ([]bool, int) {
	w, h := cal.Largeur, cal.Hauteur
	mpp := cal.MetresParPixel
	// Une composante garde son bit a partir de ce nombre de pixels : n * mpp2 >= AireMinAnneau.
	pixelsMin := int(math.Ceil(AireMinAnneau/(mpp*mpp) - 1e-9))
	out := make([]bool, len(masque))
	vu := make([]bool, len(masque))
	file := make([]int32, 0, 1024)
	eteints := 0
	for debut, ok := range masque {
		if !ok || vu[debut] {
			continue
		}
		file = file[:0]
		file = append(file, int32(debut))
		vu[debut] = true
		for tete := 0; tete < len(file); tete++ {
			file = voisinsDeLaZone(file, int(file[tete]), w, h, masque, vu)
		}
		if len(file) < pixelsMin {
			eteints += len(file)
			continue
		}
		for _, k := range file {
			out[k] = true
		}
	}
	return out, eteints
}

// voisinsDeLaZone ajoute a `file` les voisins a 4 du pixel `k` qui sont dans le masque et pas encore
// vus, et les marque vus.
func voisinsDeLaZone(file []int32, k, w, h int, masque, vu []bool) []int32 {
	i, j := k%w, k/w
	pousse := func(n int) {
		if masque[n] && !vu[n] {
			vu[n] = true
			file = append(file, int32(n))
		}
	}
	if i > 0 {
		pousse(k - 1)
	}
	if i < w-1 {
		pousse(k + 1)
	}
	if j > 0 {
		pousse(k - w)
	}
	if j < h-1 {
		pousse(k + w)
	}
	return file
}
