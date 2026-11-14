package components

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func navLinkClass(current, href string) string {
	base := "rounded-full px-4 py-2 text-sm font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-2"
	if navActive(current, href) {
		return base + " bg-slate-900 text-white shadow-sm"
	}
	return base + " text-slate-600 hover:bg-slate-100 hover:text-slate-950"
}

func navActive(current, href string) bool {
	if href == "/" {
		return current == "/"
	}
	return current == href || strings.HasPrefix(current, href+"/")
}

func navAria(current, href string) string {
	if navActive(current, href) {
		return "page"
	}
	return ""
}

func cartBadgeText(count int) string {
	if count > 99 {
		return "99+"
	}
	return strconv.Itoa(count)
}

func cartBadgeClass(count int) string {
	base := "inline-flex min-w-5 items-center justify-center rounded-full px-1.5 py-0.5 text-[11px] font-bold leading-none"
	if count > 0 {
		return base + " bg-emerald-600 text-white"
	}
	return base + " bg-slate-200 text-slate-600"
}

func cartItemPath(productID string) string {
	return "/cart/items/" + url.PathEscape(productID)
}

func cartQuantityVals(quantity int) string {
	return fmt.Sprintf(`{"quantity":%d}`, quantity)
}

func stockLabel(stock int, tracked bool) string {
	if !tracked {
		return "Available"
	}
	if stock <= 0 {
		return "Out of stock"
	}
	if stock < 10 {
		return "Only " + strconv.Itoa(stock) + " left"
	}
	return "In stock"
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	return strings.TrimSpace(string([]rune(value)[:limit])) + "…"
}

func paragraphLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	parts := strings.Split(value, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
