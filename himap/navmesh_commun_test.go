package himap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArmeReferenceDepuisNavmeshAppeleDuCommunSeul : la reference prise sur le maillage ne
// s'arme qu'a un endroit, `appliqueNavmesh`. Deux chaines qui l'armaient chacune de leur cote
// divergeaient (bascule `NiveauHautNavmesh` oubliee, journal different) ; ce test relit les
// sources hors tests et refuse tout appel ailleurs que dans navmesh_commun.go (l'appelant) et
// reference_navmesh.go (la definition).
func TestArmeReferenceDepuisNavmeshAppeleDuCommunSeul(t *testing.T) {
	autorises := map[string]bool{"navmesh_commun.go": true, "reference_navmesh.go": true}
	fichiers, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	lus := 0
	for _, f := range fichiers {
		if strings.HasSuffix(f, "_test.go") || autorises[f] {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lus++
		for i, ligne := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(ligne), "//") {
				continue
			}
			if strings.Contains(ligne, "ArmeReferenceDepuisNavmesh(") {
				t.Errorf("%s:%d : appel a ArmeReferenceDepuisNavmesh hors de navmesh_commun.go — passer par appliqueNavmesh", f, i+1)
			}
		}
	}
	if lus == 0 {
		t.Fatal("aucune source lue : le garde-rail ne garde rien")
	}
}
