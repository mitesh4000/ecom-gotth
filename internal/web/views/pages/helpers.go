package pages

import (
	"strings"

	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

func paragraphs(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func galleryImages(images []string) []string {
	if len(images) > 4 {
		return images[:4]
	}
	return images
}

func quantityMax(product service.StoreProduct) int {
	if product.StockTracked && product.Stock < service.MaxCartQuantity {
		if product.Stock < 1 {
			return 1
		}
		return product.Stock
	}
	return service.MaxCartQuantity
}
