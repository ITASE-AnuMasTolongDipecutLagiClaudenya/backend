package nutrition

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type Catalog struct {
	byName    map[string]CatalogItem
	byBarcode map[string]CatalogItem
}

type CatalogItem struct {
	Name         string
	Manufacturer string
	Barcode      string
	Calories     float64
	Protein      float64
	Fat          float64
	Carbs        float64
	Sugar        *float64
	Sodium       *float64
}

func LoadCatalog(r io.Reader) (*Catalog, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read nutrition catalog header: %w", err)
	}
	columns := make(map[string]int, len(headers))
	for i, header := range headers {
		columns[strings.TrimSpace(header)] = i
	}
	for _, required := range []string{"name", "name_clean", "manufacturer", "barcode", "basis_gizi", "energy_kcal", "protein_g", "fat_g", "carbohydrate_g", "sugar_g", "sodium_mg", "kualitas", "bisa_disajikan"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("nutrition catalog missing column %q", required)
		}
	}

	catalog := &Catalog{byName: map[string]CatalogItem{}, byBarcode: map[string]CatalogItem{}}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read nutrition catalog row %d: %w", line, err)
		}
		value := func(key string) string { return strings.TrimSpace(row[columns[key]]) }
		if value("bisa_disajikan") != "1" || value("basis_gizi") != "per_100g" || (value("kualitas") != "ok" && value("kualitas") != "natrium_dikoreksi") {
			continue
		}
		name := value("name")
		key := normalizeCatalogName(value("name_clean"))
		if key == "" {
			key = normalizeCatalogName(name)
		}
		if name == "" || key == "" {
			continue
		}
		calories, okEnergy := catalogNumber(value("energy_kcal"))
		protein, okProtein := catalogNumber(value("protein_g"))
		fat, okFat := catalogNumber(value("fat_g"))
		carbs, okCarbs := catalogNumber(value("carbohydrate_g"))
		if !okEnergy || !okProtein || !okFat || !okCarbs {
			continue
		}
		item := CatalogItem{
			Name: name, Manufacturer: value("manufacturer"), Barcode: value("barcode"),
			Calories: calories, Protein: protein, Fat: fat, Carbs: carbs,
			Sugar: catalogOptionalNumber(value("sugar_g")), Sodium: catalogOptionalNumber(value("sodium_mg")),
		}
		if _, exists := catalog.byName[key]; !exists {
			catalog.byName[key] = item
		}
		if barcode := normalizeBarcode(item.Barcode); barcode != "" {
			if _, exists := catalog.byBarcode[barcode]; !exists {
				catalog.byBarcode[barcode] = item
			}
		}
	}
	return catalog, nil
}

func (c *Catalog) Match(name, barcode string) *CatalogItem {
	if c == nil {
		return nil
	}
	if key := normalizeBarcode(barcode); key != "" {
		if item, ok := c.byBarcode[key]; ok {
			return &item
		}
	}
	if item, ok := c.byName[normalizeCatalogName(name)]; ok {
		return &item
	}
	return nil
}

func normalizeCatalogName(value string) string {
	var result strings.Builder
	space := true
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
			space = false
		} else if !space {
			result.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(result.String())
}

func normalizeBarcode(value string) string {
	var result strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func catalogNumber(value string) (float64, bool) {
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) && number >= 0
}

func catalogOptionalNumber(value string) *float64 {
	if number, ok := catalogNumber(value); ok {
		return &number
	}
	return nil
}
