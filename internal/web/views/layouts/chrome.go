package layouts

// Chrome is the request-specific data every page needs in its document shell.
type Chrome struct {
	CSRFToken string
	CartCount int
	Path      string
}

// htmxHeaders is embedded on the body so every HTMX request carries the
// double-submit token. It is deliberately not an authentication signal.
func htmxHeaders(token string) string {
	return `{"X-CSRF-Token":"` + token + `"}`
}

// htmxConfig allows HTMX to swap 4xx responses so validation errors remain
// visible, while 5xx responses are not rendered into the page.
func htmxConfig() string {
	return `{"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"4..","swap":true,"error":true},{"code":"5..","swap":false,"error":true}]}`
}
