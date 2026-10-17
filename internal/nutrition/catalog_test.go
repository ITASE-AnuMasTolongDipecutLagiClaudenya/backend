package nutrition

import (
	"strings"
	"testing"
)

func TestApplyCatalogIsExactAndUsesCatalogValuesPerPortion(t *testing.T) {
	catalog, err := LoadCatalog(strings.NewReader(`id,name,name_clean,manufacturer,barcode,basis_gizi,energy_kcal,protein_g,fat_g,carbohydrate_g,sugar_g,sodium_mg,kualitas,bisa_disajikan,sumber_grup
1,Nasi putih,nasi putih,,,per_100g,175,4,0.5,38,0.1,2,ok,1,base
 2,Nasi perkiraan,nasi perkiraan,,,per_100g_perkiraan,180,4,1,40,1,2,ok,1,base
 3,Nasi belum pasti,nasi belum pasti,,,tidak_pasti,190,4,1,41,1,2,ok,1,base
`))
	if err != nil {
		t.Fatal(err)
	}
	items := []map[string]any{
		{"name": "Nasi putih", "portion_g": 150, "serving_size": "150 g", "calories": 999},
		{"name": "Nasi putih spesial", "portion_g": 100},
		{"name": "Nasi perkiraan", "portion_g": 100},
		{"name": "Nasi belum pasti", "portion_g": 100},
		{"name": "Makanan tidak ada", "portion_g": 100},
	}
	resolved, unmatched := ApplyCatalog(catalog, items)
	if resolved[0]["nutrition_source"] != "catalog" || resolved[0]["calories"] != 262.5 || resolved[0]["protein_g"] != 6.0 {
		t.Fatalf("catalog result = %#v", resolved[0])
	}
	if resolved[0]["sugar_g"] != 0.15 || resolved[0]["sodium_mg"] != 3.0 {
		t.Fatalf("scaled micronutrients = %#v", resolved[0])
	}
	if len(resolved) != 4 || resolved[1]["nutrition_source"] != "catalog_estimate" || resolved[1]["calories"] != float64(175) || resolved[2]["nutrition_source"] != "catalog_estimate" || resolved[2]["calories"] != float64(180) || resolved[3]["nutrition_source"] != "catalog_estimate" || resolved[3]["calories"] != float64(190) || len(unmatched) != 1 || unmatched[0]["nutrition_source"] != "estimate" {
		t.Fatalf("unknowns should use estimate source: resolved=%#v unmatched=%#v", resolved, unmatched)
	}
}

func TestApplyCatalogDoesNotPretendUnknownNutrientsAreZero(t *testing.T) {
	catalog, err := LoadCatalog(strings.NewReader(`id,name,name_clean,manufacturer,barcode,basis_gizi,energy_kcal,protein_g,fat_g,carbohydrate_g,sugar_g,sodium_mg,kualitas,bisa_disajikan,sumber_grup
1,Apel,apel,,,per_100g,52,0.3,0.2,14,,,ok,1,base
`))
	if err != nil {
		t.Fatal(err)
	}
	resolved, _ := ApplyCatalog(catalog, []map[string]any{{"name": "Apel"}})
	if resolved[0]["sugar_g"] != nil || resolved[0]["sodium_mg"] != nil {
		t.Fatalf("missing dataset fields must remain unknown: %#v", resolved[0])
	}
	if resolved[0]["serving_size"] != "100 g" {
		t.Fatalf("default catalog basis should be explicit: %#v", resolved[0])
	}
}
