package webhandlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/session"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/components"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/pages"
)

// CartPage renders the full cart. The owner is resolved from the server-side
// session, never from the URL, so one browser cannot request another cart.
func (h *Handler) CartPage(w http.ResponseWriter, r *http.Request) {
	owner := session.FromContext(r.Context()).Owner()
	cart, err := h.cart().Get(r.Context(), owner)
	if err != nil {
		slog.Error("web: load cart", "error", err)
		h.Renderer.Error(w, r, http.StatusInternalServerError, "Your cart could not be loaded.")
		return
	}
	h.Renderer.Page(w, r, http.StatusOK, "Cart", pages.Cart(cart))
}

// AddCartItem implements POST /cart/items. It accepts only a product id and
// quantity; prices, totals and owner ids from the client are ignored.
func (h *Handler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	productID := strings.TrimSpace(r.FormValue("productId"))
	quantity := 1
	if raw := strings.TrimSpace(r.FormValue("quantity")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.cartError(w, r, service.ErrInvalidQuantity)
			return
		}
		quantity = parsed
	}

	err := h.cart().Add(r.Context(), session.FromContext(r.Context()).Owner(), productID, quantity)
	if err != nil {
		h.cartError(w, r, err)
		return
	}
	cart := h.reloadCart(w, r)
	h.cartSuccess(w, r, cart, "Added "+cartItemName(cart, productID)+" to your cart.")
}

// UpdateCartItem implements PATCH and PUT /cart/items/{productID}.
func (h *Handler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	productID := strings.TrimSpace(chi.URLParam(r, "productID"))
	quantity, err := strconv.Atoi(strings.TrimSpace(r.FormValue("quantity")))
	if err != nil {
		h.cartError(w, r, service.ErrInvalidQuantity)
		return
	}
	err = h.cart().SetQuantity(r.Context(), session.FromContext(r.Context()).Owner(), productID, quantity)
	if err != nil {
		h.cartError(w, r, err)
		return
	}
	cart := h.reloadCart(w, r)
	h.cartSuccess(w, r, cart, "Quantity updated.")
}

// RemoveCartItem implements DELETE /cart/items/{productID}.
func (h *Handler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	productID := strings.TrimSpace(chi.URLParam(r, "productID"))
	if err := h.cart().Remove(r.Context(), session.FromContext(r.Context()).Owner(), productID); err != nil {
		h.cartError(w, r, err)
		return
	}
	cart := h.reloadCart(w, r)
	h.cartSuccess(w, r, cart, "Item removed from your cart.")
}

func (h *Handler) cart() *service.CartService {
	if h.Cart != nil {
		return h.Cart
	}
	return service.NewCart(h.DB)
}

func (h *Handler) reloadCart(w http.ResponseWriter, r *http.Request) service.CartView {
	cart, err := h.cart().Get(r.Context(), session.FromContext(r.Context()).Owner())
	if err != nil {
		// The mutation already succeeded. Returning an empty view would be
		// misleading, so log and let the error path render a safe message.
		slog.Error("web: reload cart after mutation", "error", err)
	}
	return cart
}

func (h *Handler) cartSuccess(w http.ResponseWriter, r *http.Request, cart service.CartView, message string) {
	if !render.IsHTMX(r) {
		h.Renderer.Redirect(w, r, "/cart")
		return
	}
	h.Renderer.Fragment(w, r, http.StatusOK, components.CartMutationResponse(cart, message))
}

func (h *Handler) cartError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := cartErrorStatus(err)
	if status >= 500 {
		slog.Error("web: cart mutation", "error", err)
	}
	if !render.IsHTMX(r) {
		h.Renderer.Error(w, r, status, message)
		return
	}
	h.Renderer.ValidationError(w, r, status, components.CartFeedback(message, true))
}

func cartErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrCartOwnerInvalid):
		return http.StatusForbidden, "Your cart session is not available. Please refresh and try again."
	case errors.Is(err, service.ErrCartProductNotFound):
		return http.StatusNotFound, "That product could not be found."
	case errors.Is(err, service.ErrProductUnavailable):
		return http.StatusConflict, "That product is no longer available."
	case errors.Is(err, service.ErrCartQuantityLimit):
		return http.StatusUnprocessableEntity, "You can add at most " + strconv.Itoa(service.MaxCartQuantity) + " of one item."
	case errors.Is(err, service.ErrInsufficientStock):
		return http.StatusConflict, "There is not enough stock for that quantity."
	case errors.Is(err, service.ErrCartItemNotFound):
		return http.StatusNotFound, "That item is no longer in your cart."
	case errors.Is(err, service.ErrInvalidQuantity):
		return http.StatusUnprocessableEntity, "Choose a quantity between 1 and " + strconv.Itoa(service.MaxCartQuantity) + "."
	default:
		return http.StatusInternalServerError, "Your cart could not be updated. Please try again."
	}
}

func cartItemName(cart service.CartView, productID string) string {
	for _, item := range cart.Items {
		if item.ProductID == productID {
			return "“" + item.Name + "”"
		}
	}
	return "the item"
}
