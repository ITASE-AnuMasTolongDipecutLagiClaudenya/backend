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
	"lotion", "kosmetik", "makeup", "skincare", "tusuk gigi", "gunting", "korek", "baterai", "battery",
	"amplop", "pulpen", "pensil", "kertas", "plastik", "kantong plastik", "pembersih", "pencuci",
	"pewangi", "deodoran", "odol", "hansplast", "masker", "hand sanitizer", "sanitizer",
	"piring", "gelas", "sendok", "garpu", "sedotan", "kipas", "lampu", "kabel",
}

func hasReceiptKeyword(name, keyword string) bool {
	normalize := func(value string) string {
		return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return ' '
		}, value)), " ")
	}
	return strings.Contains(" "+normalize(name)+" ", " "+normalize(keyword)+" ")
}

func IsNonFood(name string) bool {
	for _, k := range nonFoodKeywords {
		if hasReceiptKeyword(name, k) {
			return true
		}
	}
	return false
}

func inferReceiptCategory(name string) string {
	for _, keyword := range []string{
		"aqua", "vit", "le minerale", "cleo", "ades", "air mineral", "teh", "tea", "kopi", "coffee",
		"susu", "milk", "soda", "cola", "jus", "juice", "sirup", "syrup", "pocari", "mizone", "yakult",
		"boba", "beverage", "drink", "fanta", "sprite", "coca-cola", "coca cola", "pepsi", "frestea",
		"fruit tea", "teh botol", "ultra milk", "ultramilk", "frisian flag", "bear brand", "indomilk", "cimory",
		"good day", "nescafe", "kapal api", "milo", "floridina", "minute maid",
	} {
		if hasReceiptKeyword(name, keyword) {
			return "minuman"
		}
	}
	for _, keyword := range []string{
		"apel", "pisang", "jeruk", "mangga", "alpukat", "semangka", "melon", "anggur", "nanas", "pepaya",
		"pir", "stroberi", "strawberry", "durian", "salak", "jambu", "kelapa", "kiwi", "manggis", "rambutan",
		"sirsak", "leci", "buah naga", "fruit",
	} {
		if hasReceiptKeyword(name, keyword) {
			return "buah"
		}
	}
	for _, keyword := range []string{
		"nasi", "roti", "mie", "mi instan", "ayam", "daging", "sapi", "ikan", "telur", "tahu", "tempe",
		"sayur", "bakso", "sosis", "nugget", "biskuit", "keripik", "snack", "camilan", "cemilan", "kue",
		"donat", "pizza", "burger", "sate", "sambal", "saus", "bumbu", "keju", "yogurt", "yoghurt",
		"cokelat", "coklat", "permen", "gula", "garam", "sereal", "oat", "bubur", "rendang", "makanan", "food",
		"kacang", "cashew", "almond", "nuts",
	} {
		if hasReceiptKeyword(name, keyword) {
			return "makanan"
		}
	}
	return ""
}

func NormalizeCategory(v any) string {
	c, _ := v.(string)
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "makanan", "food", "meal", "meals", "snack", "snacks", "cemilan", "camilan", "makanan ringan", "makanan kemasan", "produk makanan", "lauk", "nasi", "roti":
		return "makanan"
	case "minuman", "drink", "drinks", "beverage", "beverages", "minum", "minuman kemasan", "minuman ringan", "produk minuman", "soft drink", "soft drinks":
		return "minuman"
	case "buah", "buah-buahan", "buah segar", "fruit", "fruits", "sayur", "vegetable":
		return "buah"
	case "":
		return ""
	default:
		return c
	}
}

// normalizeNutritionAliases maps common alternative nutrition field names
// (including a nested gizi/`gizi_per_sajian` object some models emit) onto the
// canonical flat keys, so values survive even when the model drifts from the
// requested schema.
func normalizeNutritionAliases(item map[string]any) {
	// Models alternate between nested containers for the same fields.
	for _, key := range []string{"gizi_per_sajian", "gizi", "nutrition", "nutrition_facts", "nutrition_per_serving"} {
		if nested, ok := item[key].(map[string]any); ok {
			for k, v := range nested {
				if _, exists := item[k]; !exists {
					item[k] = v
				}
			}
			delete(item, key)
		}
	}
	alias := map[string]string{
		"kalori":       "calories",
		"energi":       "calories",
		"energy":       "calories",
		"protein":      "protein_g",
		"karbohidrat":  "carbs_g",
		"karbo":        "carbs_g",
		"carbohydrate": "carbs_g",
		"lemak":        "fat_g",
		"fat":          "fat_g",
		"gula":         "sugar_g",
		"natrium":      "sodium_mg",
	}
	for from, to := range alias {
		if v, ok := item[from]; ok {
			if _, exists := item[to]; !exists {
				item[to] = v
			}
			delete(item, from)
		}
	}
}

func NormalizeScanItems(result map[string]any, mode string) []map[string]any {
	items := []map[string]any{}
	collect := func(entry any) {
		item, ok := entry.(map[string]any)
		if !ok {
			return
		}
		normalizeNutritionAliases(item)
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		// Receipts abbreviate product names ("KANZLR BAKSO HOT"). Keep the
		// printed name for display and use the LLM-expanded guess for lookup.
		if guess, _ := item["product_guess"].(string); strings.TrimSpace(guess) != "" {
			if name != "" {
				item["raw_name"] = name
			}
			name = strings.TrimSpace(guess)
		}
		delete(item, "product_guess")
		if confidence, _ := item["name_confidence"].(string); confidence != "high" && confidence != "medium" && confidence != "low" {
			delete(item, "name_confidence")
		}
		if name == "" || IsNonFood(name) {
			return
		}
		item["name"] = name
		rawCategory, _ := item["category"].(string)
		category := NormalizeCategory(rawCategory)
		if mode == "receipt" && category != "makanan" && category != "minuman" && category != "buah" {
			category = inferReceiptCategory(name)
			if category == "" {
				category = "makanan"
			}
		}
		if category == "makanan" || category == "minuman" || category == "buah" {
			item["category"] = category
		} else {
			delete(item, "category")
		}
		for _, k := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			item[k] = ToFloatOrNil(item[k])
		}
		if source, _ := item["nutrition_source"].(string); source != "catalog" && source != "estimate" && source != "label" {
			delete(item, "nutrition_source")
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
	for _, k := range []string{"name", "raw_name", "name_confidence", "_estimate_id", "category", "serving_size", "calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg", "sources", "note", "mode", "user_id"} {
		if v, ok := result[k]; ok {
			item[k] = v
		}
	}
	if name, _ := item["name"].(string); strings.TrimSpace(name) != "" && mode != "receipt" {
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
