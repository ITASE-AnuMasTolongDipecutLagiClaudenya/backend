package nutrition

import (
	"math"
	"strconv"
	"strings"
)

func ApplyCatalog(catalog *Catalog, items []map[string]any) ([]map[string]any, []map[string]any) {
	resolved := make([]map[string]any, 0, len(items))
	unmatched := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		item := make(map[string]any, len(raw)+2)
		for key, value := range raw {
			item[key] = value
		}
		name, _ := item["name"].(string)
		barcode, _ := item["barcode"].(string)
		match := catalog.Match(name, barcode)
		if match == nil {
			item["nutrition_source"] = "estimate"
			unmatched = append(unmatched, item)
			continue
		}
		portion := 100.0
		if grams, ok := ToFloatOrNil(item["portion_g"]).(float64); ok && grams > 0 {
			portion = grams
		}
		factor := portion / 100
		item["name"] = match.Name
		if match.Manufacturer != "" {
			item["manufacturer"] = match.Manufacturer
		}
		if match.Barcode != "" {
			item["barcode"] = match.Barcode
		}
		if strings.TrimSpace(fmtString(item["serving_size"])) == "" {
			item["serving_size"] = formatPortion(portion)
		}
		item["calories"] = roundCatalog(match.Calories * factor)
		item["protein_g"] = roundCatalog(match.Protein * factor)
		item["fat_g"] = roundCatalog(match.Fat * factor)
		item["carbs_g"] = roundCatalog(match.Carbs * factor)
		item["sugar_g"] = scaleOptional(match.Sugar, factor)
		item["sodium_mg"] = scaleOptional(match.Sodium, factor)
		item["nutrition_source"] = "catalog"
		resolved = append(resolved, item)
	}
	return resolved, unmatched
}

func fmtString(value any) string {
	text, _ := value.(string)
	return text
}

func scaleOptional(value *float64, factor float64) any {
	if value == nil {
		return nil
	}
	return roundCatalog(*value * factor)
}

func roundCatalog(value float64) float64 {
	return math.Round(value*100) / 100
}

func formatPortion(grams float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(grams, 'f', 2, 64), "0"), ".") + " g"
}
