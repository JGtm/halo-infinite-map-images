package himap

// sddt_partage.go — LA ZONE JOUABLE ET L'EAU D'UNE CARTE DONT LA GEOMETRIE VIT DANS UN MODULE PARTAGE.
//
// Le tag sddt d'un module partage (`any/globals/<nom>`, lu depuis `ModuleVariante`) decrit TOUTES
// les arenes du module, pas celle de la carte cuite. Mesure sur Live Fire
// (`any/globals/common-rtx-new.module`, 28 ancres, arene du bsp retenu par ChoisitBSP, maillage de
// la ressource 20390) :
//
//	un seul tag sddt : 124 triangles-frontieres en UNE composante connexe (X [-20,7 ; 101,6]
//	Y [-10,5 ; 105,0]), dont 74 seulement ont leur centroide dans la boite de l'arene retenue ;
//	42 volumes d'eau, les uns sous le sol de Live Fire, les autres sous les arenes voisines.
//
// LA FRONTIERE : une seule surface fermee et non convexe. Elle ne se decoupe pas par arene (ouvrir
// la surface casse la parite de rayon de ContientFrontiere). Entiere, elle garde les 28 ancres et
// 99,63 % de l'empreinte du maillage, mais retire 46 cellules de 0,092 m (0,4 m2) du praticable,
// une bande de 0,3 m au bord est (X 29,31 a 29,59, Y 22,66 a 28,46), pour 253 cellules hors
// empreinte retirees a 0,09 a 0,65 m du maillage : ce que le rognage au maillage (marge 0,5 m)
// fait deja. Aucun gain, un praticable rogne : elle n'est pas appliquee.
//
// L'EAU : 42 volumes dont les toits sont a -2,8 m et -5,9 m, sous le sol joue (-0,07 m) et sous le
// plancher du maillage (-1,0 m). PoseEau ne protege que les cellules qui ont de la matiere : elle
// peint 17 581 cellules (149 m2) sur les cellules sans matiere, dont 84 % (14 831) tombent sur le
// praticable du maillage. Live Fire n'a pas d'eau a l'ecran : elle n'est pas posee.

import "context"

// raisonFrontierePartagee et raisonEauPartagee sont les degradations publiees au sidecar : ce que
// le module partage porte et ne se pose pas, avec la mesure.
const (
	raisonFrontierePartagee = "frontiere de mort du module partage NON appliquee : un seul volume ferme couvre l'arene " +
		"et ses voisines, il retire 0,4 m2 du praticable du maillage sans rien gagner sur le rognage au maillage"
	raisonEauPartagee = "eau du module partage NON posee : 42 volumes sous le sol joue, 84 % de leurs cellules " +
		"tomberaient sur le praticable du maillage"
)

// sddtPartageIgnore dit si le module de geometrie est un module partage, et inscrit alors la
// degradation `raison` au bilan : son sddt n'est pas celui de la carte.
func sddtPartageIgnore(ctx context.Context, b *BilanCuisson, cheminModule, raison string) bool {
	if _, partage := cheminVarianteGlobale(cheminModule, "any"); !partage {
		return false
	}
	b.degrade(ctx, "%s", raison)
	return true
}
