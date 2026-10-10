package nutrition

import (
	"encoding/json"
	"strconv"
	"strings"
)

var nonFoodKeywords = []string{
	"pajak", "ppn", "diskon", "discount", "total", "subtotal", "sub total", "kembalian", "kembali",
	"tunai", "cash", "debit", "kredit", "credit", "qr", "qris", "layanan", "service", "kantong",
	"sabun", "sampo", "shampoo", "deterjen", "detergen", "pasta gigi", "sikat gigi", "tisu", "tissue",
	"popok", "diapers", "pembalut", "rokok", "cigarette", "pulsa", "voucher", "token", "parfum",
	"lotion", "kosmetik", "makeup", "skincare",
}

func IsNonFood(name string) bool {
	lower := strings.ToLower(name)
	for _, k := range nonFoodKeywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func NormalizeCategory(v any) string {
	c, _ := v.(string)
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "makanan", "food", "meal", "meals", "snack", "snacks", "cemilan", "camilan", "makanan ringan", "makanan kemasan", "produk makanan", "lauk", "nasi", "roti":
		return "makanan"
	case "minuman", "drink", "drinks", "beverage", "beverages", "minum", "minuman kemasan", "produk minuman":
		return "minuman"
	case "buah", "buah-buahan", "buah segar", "fruit", "fruits", "sayur", "vegetable":
		return "buah"
	case "":
		return ""
	default:
		return c
	}
}

func NormalizeScanItems(result map[string]any) []map[string]any {
	items := []map[string]any{}
	collect := func(entry any) {
		item, ok := entry.(map[string]any)
		if !ok {
			return
		}
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" || IsNonFood(name) {
			return
		}
		item["name"] = name
		if c, ok := item["category"].(string); ok && strings.TrimSpace(c) != "" {
			c = NormalizeCategory(c)
			if c == "makanan" || c == "minuman" || c == "buah" {
				item["category"] = c
			} else {
				delete(item, "category")
			}
		}
		for _, k := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			item[k] = ToFloatOrNil(item[k])
		}
		items = append(items, item)
	}
	if list, ok := result["items"].([]any); ok {
		for _, e := range list {
			collect(e)
		}
		return items
	}
	if list, ok := result["items"].([]map[string]any); ok {
		for _, e := range list {
			collect(e)
		}
		return items
	}
	item := map[string]any{}
	for _, k := range []string{"name", "category", "serving_size", "calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg", "note", "mode", "user_id"} {
		if v, ok := result[k]; ok {
			item[k] = v
		}
	}
	if name, _ := item["name"].(string); strings.TrimSpace(name) != "" {
		for _, k := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			item[k] = ToFloatOrNil(item[k])
		}
		items = append(items, item)
	}
	return items
}

func ToFloatOrNil(v any) any {
	switch n := v.(type) {
	case nil:
		return nil
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return nil
		}
		return f
	case string:
		cleaned := strings.TrimSpace(n)
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' || r == '.' || r == '-' {
				return r
			}
			return -1
		}, cleaned)
		if digits == "" || digits == "-" || digits == "." {
			return nil
		}
		if f, err := strconv.ParseFloat(digits, 64); err == nil {
			return f
		}
		return nil
	default:
		return nil
	}
}
