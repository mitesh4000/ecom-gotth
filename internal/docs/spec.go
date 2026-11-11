package docs

import (
	"strings"
	"unicode"
)

// Spec builds the OpenAPI 3 document served at /swagger/openapi.json.
func Spec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Pusti Shangar API",
			"version":     "1.0.0",
			"description": infoDescription,
		},
		"servers": []any{
			map[string]any{"url": "/", "description": "Current server"},
		},
		"tags":  tagList(),
		"paths": paths(),
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "JWT returned by /api/login or /customer/login.",
				},
				"cookieAuth": map[string]any{
					"type":        "apiKey",
					"in":          "cookie",
					"name":        "token",
					"description": "JWT cookie set by the login endpoints.",
				},
			},
			"schemas": schemas(),
		},
	}
}

const infoDescription = "HTTP API for the Pusti Shangar storefront, admin panel, and supporting services.\n\n" +
	"**Authentication** – Login endpoints return a JWT. Send it as `Authorization: Bearer <token>`, " +
	"as the `token` cookie, or as a `token` field in the JSON body. The **Authorize** button in Swagger UI " +
	"sets the Bearer header for the requests you try from this page.\n\n" +
	"**Content types** – JSON is used by default. Endpoints that upload images also accept " +
	"`multipart/form-data`; their request body is documented with that content type.\n\n" +
	"**Responses** – Many handlers return HTTP 200 even for application-level failures. Always check the " +
	"`success` field (`false` indicates an error) and the accompanying `msg`/`error` field.\n\n" +
	"**IDs** – Records expose both `id` and `_id` with the same value for frontend compatibility."

func tagList() []any {
	tags := []struct {
		name string
		desc string
	}{
		{"Users", "Admin and manager accounts, login, and password management."},
		{"Roles", "Roles and permissions assigned to users."},
		{"Payments", "Payment gateway placeholders."},
		{"Categories", "Top-level catalog categories."},
		{"Subcategories", "Second-level catalog categories."},
		{"Sub-subcategories", "Third-level catalog categories."},
		{"Products", "Products, variants, pricing, and images."},
		{"Customers", "Storefront customers, carts, and wishlists."},
		{"Orders", "Orders, invoices, and sales reports."},
		{"Dashboard", "Aggregated dashboard statistics."},
		{"Stocks", "Inventory records and stock lookups."},
		{"Coupons", "Discount coupons."},
		{"GST", "Tax rates."},
		{"Daily Rates", "Daily price updates."},
		{"Masters", "Colors, materials, seasons, and sizes."},
		{"Banners", "Homepage and promotional banners."},
		{"Blogs", "Blog posts."},
		{"Blog Categories", "Blog categories."},
		{"Content", "Editable content blocks."},
		{"Gallery", "Gallery categories."},
		{"Gallery Items", "Images inside gallery categories."},
		{"Contact", "Contact form submissions."},
		{"Subscribe", "Newsletter subscribers."},
	}

	out := make([]any, 0, len(tags))
	for _, t := range tags {
		out = append(out, map[string]any{"name": t.name, "description": t.desc})
	}
	return out
}

type operation struct {
	method  string
	path    string
	tag     string
	summary string
	desc    string
	req     string
	resp    string
	form    bool
	auth    bool
}

func op(method, path, tag, summary, req, resp string, form, auth bool) operation {
	return operation{method: method, path: path, tag: tag, summary: summary, req: req, resp: resp, form: form, auth: auth}
}

func opDesc(method, path, tag, summary, desc, req, resp string, form, auth bool) operation {
	return operation{method: method, path: path, tag: tag, summary: summary, desc: desc, req: req, resp: resp, form: form, auth: auth}
}

var operations = []operation{
	// Users
	op("POST", "/api/register", "Users", "Register a user", "UserCreate", "UserResponse", true, false),
	op("POST", "/api/login", "Users", "Log in a user", "LoginRequest", "LoginUserResponse", false, false),
	op("POST", "/api/logout", "Users", "Log out a user", "", "MessageResponse", false, false),
	opDesc("POST", "/api/getloggedinuser", "Users", "Get the logged-in user",
		"The token may come from the Authorization header, the token cookie, or a token body field.",
		"TokenRequest", "UserResponse", false, true),
	op("POST", "/api/getspecificuser", "Users", "Get a user by id", "IDRequest", "UserResponse", false, false),
	op("POST", "/api/loggedin", "Users", "Check user login status", "TokenRequest", "SuccessResponse", false, false),
	opDesc("POST", "/api/updateuser", "Users", "Update a user",
		"Accepts multipart/form-data when a new profile photo is uploaded.",
		"UserUpdate", "UserResponse", true, true),
	opDesc("POST", "/api/updatepassword", "Users", "Change the user password",
		"Uses the logged-in user unless an id is supplied in the body.",
		"UpdatePasswordRequest", "MessageResponse", false, true),
	op("POST", "/api/forgotpassword", "Users", "Request a user password reset", "EmailRequest", "MessageResponse", false, false),
	op("POST", "/api/resetpassword/{resetToken}", "Users", "Reset a user password with a token", "PasswordRequest", "MessageResponse", false, false),
	op("POST", "/api/getusers", "Users", "List users", "", "UsersResponse", false, false),
	op("POST", "/api/deleteuser", "Users", "Delete a user", "IDRequest", "MessageResponse", false, false),

	// Roles
	op("POST", "/api/getroles", "Roles", "List roles", "", "RolesResponse", false, false),
	op("POST", "/api/createrole", "Roles", "Create a role", "RoleCreate", "RoleResponse", false, false),
	op("POST", "/api/deleterole", "Roles", "Delete a role", "IDRequest", "MessageResponse", false, false),

	// Payments
	op("POST", "/api/payment", "Payments", "Payment gateway placeholder", "", "MessageResponse", false, false),
	op("POST", "/api/create-order", "Payments", "Create a payment order placeholder", "", "MessageResponse", false, false),

	// Categories
	op("POST", "/category/addcategory", "Categories", "Create a category", "CategoryCreate", "CategoryResponse", true, false),
	op("POST", "/category/getcategories", "Categories", "List categories", "", "CategoriesResponse", false, false),
	op("POST", "/category/getspecificcategory/{id}", "Categories", "Get a category", "", "CategoryResponse", false, false),
	op("POST", "/category/updatecategory/{id}", "Categories", "Update a category", "CategoryUpdate", "CategoryResponse", true, false),
	op("POST", "/category/deletecategory/{id}", "Categories", "Delete a category", "", "MessageResponse", false, false),
	op("POST", "/category/getsubandsubsub/{id}", "Categories", "Get subcategories and sub-subcategories for a category", "", "SubAndSubSubResponse", false, false),

	// Subcategories
	op("POST", "/subcategory/addsubcategory", "Subcategories", "Create a subcategory", "SubCategoryCreate", "SubCategoryResponse", false, false),
	op("POST", "/subcategory/getsubcategories", "Subcategories", "List subcategories", "", "SubCategoriesResponse", false, false),
	op("POST", "/subcategory/getsubspecificcategory/{id}", "Subcategories", "Get a subcategory", "", "SubCategoryResponse", false, false),
	op("POST", "/subcategory/updatesubcategory/{id}", "Subcategories", "Update a subcategory", "SubCategoryUpdate", "SubCategoryResponse", false, false),
	op("POST", "/subcategory/deletesubcategory/{id}", "Subcategories", "Delete a subcategory", "", "MessageResponse", false, false),

	// Sub-subcategories
	op("POST", "/subsubcategory/addsubsubcategory", "Sub-subcategories", "Create a sub-subcategory", "SubSubCategoryCreate", "SubSubCategoryResponse", false, false),
	op("POST", "/subsubcategory/getsubsubcategories", "Sub-subcategories", "List sub-subcategories", "", "SubSubCategoriesResponse", false, false),
	op("POST", "/subsubcategory/getsubsubspecificcategory/{id}", "Sub-subcategories", "Get a sub-subcategory", "", "SubSubCategoryResponse", false, false),
	op("POST", "/subsubcategory/updatesubsubcategory/{id}", "Sub-subcategories", "Update a sub-subcategory", "SubSubCategoryUpdate", "SubSubCategoryResponse", false, false),
	op("POST", "/subsubcategory/deletesubsubcategory/{id}", "Sub-subcategories", "Delete a sub-subcategory", "", "MessageResponse", false, false),

	// Products
	op("POST", "/product/addproduct", "Products", "Create a product", "ProductCreate", "ProductResponse", true, false),
	op("POST", "/product/getallproducts", "Products", "List active products", "", "ProductsResponse", false, false),
	op("POST", "/product/get-products-for-table", "Products", "List all products for the admin table", "", "ProductsResponse", false, false),
	op("POST", "/product/addvar", "Products", "Create a product variant", "ProductCreate", "ProductResponse", true, false),
	op("POST", "/product/getvarproduct/{id}", "Products", "List variants of a product", "", "ProductsResponse", false, false),
	op("POST", "/product/getvarproduct", "Products", "List all product variants", "", "ProductsResponse", false, false),
	op("POST", "/product/getspecificproduct/{id}", "Products", "Get a product", "", "ProductResponse", false, false),
	op("POST", "/product/updateproduct/{id}", "Products", "Update a product", "ProductUpdate", "ProductResponse", true, false),
	op("POST", "/product/getproductsbycategoryid/{id}", "Products", "List products by category", "", "ProductsResponse", false, false),
	op("POST", "/product/deleteproduct/{id}", "Products", "Delete a product", "", "MessageResponse", false, false),
	op("POST", "/product/getproductbytags", "Products", "List products by tag", "TagRequest", "ProductsResponse", false, false),
	op("POST", "/product/getproductsbysubcategories/{id}", "Products", "List products by subcategory", "", "ProductsResponse", false, false),
	op("POST", "/product/getproductsbysubsubcategories/{id}", "Products", "List products by sub-subcategory", "", "ProductsResponse", false, false),

	// Customers
	op("POST", "/customer/register", "Customers", "Register a customer", "CustomerCreate", "CustomerResponse", false, false),
	op("POST", "/customer/login", "Customers", "Log in a customer", "LoginRequest", "CustomerLoginResponse", false, false),
	op("POST", "/customer/logout", "Customers", "Log out a customer", "", "MessageResponse", false, false),
	op("POST", "/customer/getcustomers", "Customers", "List customers", "", "CustomersResponse", false, false),
	opDesc("POST", "/customer/getloggedincustomer", "Customers", "Get the logged-in customer",
		"The token may come from the Authorization header, the token cookie, or a token body field.",
		"TokenRequest", "CustomerResponse", false, true),
	op("POST", "/customer/getspecificcustomer/{id}", "Customers", "Get a customer", "", "CustomerResponse", false, false),
	op("POST", "/customer/loggedin", "Customers", "Check customer login status", "TokenRequest", "SuccessResponse", false, false),
	op("POST", "/customer/updatecustomer/{id}", "Customers", "Update a customer", "CustomerUpdate", "CustomerResponse", false, false),
	op("POST", "/customer/updatepassword/{id}", "Customers", "Change the customer password", "UpdatePasswordRequest", "MessageResponse", false, false),
	op("POST", "/customer/forgotpassword", "Customers", "Request a customer password reset", "EmailRequest", "MessageResponse", false, false),
	op("POST", "/customer/resetpassword/{resetToken}", "Customers", "Reset a customer password with a token", "PasswordRequest", "MessageResponse", false, false),
	op("POST", "/customer/deletecustomer", "Customers", "Delete a customer", "IDRequest", "MessageResponse", false, false),
	op("POST", "/customer/addtocart/{id}", "Customers", "Add a product to the cart", "CartAddRequest", "CustomerResponse", false, false),
	op("POST", "/customer/getcart/{id}", "Customers", "Get a customer's cart", "", "CartResponse", false, false),
	op("POST", "/customer/removefromcart/{id}", "Customers", "Remove a product from the cart", "ProductIDRequest", "CustomerResponse", false, false),
	op("POST", "/customer/addtowishlist/{id}", "Customers", "Add a product to the wishlist", "ProductIDRequest", "CustomerResponse", false, false),
	op("POST", "/customer/removefromwishlist/{id}", "Customers", "Remove a product from the wishlist", "ProductIDRequest", "CustomerResponse", false, false),

	// Orders
	op("POST", "/orders/addorder", "Orders", "Create an order", "OrderCreate", "OrderResponse", false, false),
	op("POST", "/orders/getallorders", "Orders", "List orders", "", "OrdersResponse", false, false),
	op("POST", "/orders/getspecificorder/{id}", "Orders", "Get an order", "", "OrderResponse", false, false),
	op("POST", "/orders/updateorder/{id}", "Orders", "Update an order", "OrderUpdate", "OrderResponse", false, false),
	op("POST", "/orders/deleteorder/{id}", "Orders", "Delete an order", "", "MessageResponse", false, false),
	op("POST", "/orders/gethighcustomerdata", "Orders", "List high-value customers", "", "ValueCustomersResponse", false, false),
	op("POST", "/orders/getmedcustomerdata", "Orders", "List medium-value customers", "", "ValueCustomersResponse", false, false),
	op("POST", "/orders/gettopsellproducts", "Orders", "List top-selling products", "", "TopProductsResponse", false, false),

	// Dashboard
	op("POST", "/dashboard/getdashboard", "Dashboard", "Get dashboard statistics", "", "DashboardResponse", false, false),
	op("POST", "/dashboard/stats", "Dashboard", "Get dashboard statistics", "", "DashboardResponse", false, false),

	// Stocks
	op("POST", "/stocks/getstocks", "Stocks", "List stock records", "", "StocksResponse", false, false),
	op("POST", "/stocks/createstock", "Stocks", "Create a stock record", "StockCreate", "StockResponse", false, false),
	op("POST", "/stocks/getspecificstock/{id}", "Stocks", "Get a stock record", "", "StockResponse", false, false),
	op("POST", "/stocks/updatestock/{id}", "Stocks", "Update a stock record", "StockUpdate", "StockResponse", false, false),
	op("POST", "/stocks/deletestocks/{id}", "Stocks", "Delete a stock record", "", "MessageResponse", false, false),
	op("POST", "/stocks/getstocksbycategory/{id}", "Stocks", "List stock by category", "", "StocksResponse", false, false),
	op("POST", "/stocks/getstocksbyweighttype/{id}", "Stocks", "List stock by weight type", "", "StocksResponse", false, false),

	// Coupons
	op("POST", "/coupons/getcoupons", "Coupons", "List coupons", "", "CouponsResponse", false, false),
	op("POST", "/coupons/createcoupon", "Coupons", "Create a coupon", "CouponCreate", "MessageResponse", false, false),
	op("POST", "/coupons/updatecoupon/{id}", "Coupons", "Update a coupon", "CouponUpdate", "MessageResponse", false, false),
	op("POST", "/coupons/deletecoupon/{id}", "Coupons", "Delete a coupon", "", "MessageResponse", false, false),

	// GST
	op("POST", "/gst/getgst", "GST", "List GST rates", "", "GSTsResponse", false, false),
	op("POST", "/gst/creategst", "GST", "Create a GST rate", "GSTCreate", "MessageResponse", false, false),
	op("POST", "/gst/updategst/{id}", "GST", "Update a GST rate", "GSTUpdate", "MessageResponse", false, false),
	op("POST", "/gst/deletegst/{id}", "GST", "Delete a GST rate", "", "MessageResponse", false, false),

	// Daily rates
	op("POST", "/dailyrates/getrates", "Daily Rates", "List daily rates", "", "RatesResponse", false, false),
	op("POST", "/dailyrates/createrate", "Daily Rates", "Create a daily rate", "RateCreate", "MessageResponse", false, false),
	op("POST", "/dailyrates/updaterate/{id}", "Daily Rates", "Update a daily rate", "RateUpdate", "MessageResponse", false, false),
	op("POST", "/dailyrates/deleterate/{id}", "Daily Rates", "Delete a daily rate", "", "MessageResponse", false, false),

	// Masters
	op("POST", "/material/getmaterial", "Masters", "List materials", "", "MaterialsResponse", false, false),
	op("POST", "/material/creatematerial", "Masters", "Create a material", "NameRequest", "MessageResponse", false, false),
	op("POST", "/season/getseasons", "Masters", "List seasons", "", "SeasonsResponse", false, false),
	op("POST", "/season/createseason", "Masters", "Create a season", "NameRequest", "MessageResponse", false, false),
	op("POST", "/color/getcolors", "Masters", "List colors", "", "ColorsResponse", false, false),
	op("POST", "/color/addcolor", "Masters", "Create a color", "ColorCreate", "MessageResponse", true, false),
	op("POST", "/color/getspecificcolor/{id}", "Masters", "Get a color", "", "ColorResponse", false, false),
	op("POST", "/color/updatecolor/{id}", "Masters", "Update a color", "NameRequest", "MessageResponse", false, false),
	op("POST", "/color/deletecolor/{id}", "Masters", "Delete a color", "", "MessageResponse", false, false),
	op("POST", "/size/getsizes", "Masters", "List sizes", "", "SizesResponse", false, false),
	op("POST", "/size/addsize", "Masters", "Create a size", "SizeCreate", "MessageResponse", false, false),
	op("POST", "/size/getspecificsize/{id}", "Masters", "Get a size", "", "SizeResponse", false, false),
	op("POST", "/size/updatesize/{id}", "Masters", "Update a size", "SizeCreate", "MessageResponse", false, false),
	op("POST", "/size/deletesize/{id}", "Masters", "Delete a size", "", "MessageResponse", false, false),

	// Banners
	op("POST", "/banner/get-banner", "Banners", "List active banners", "", "BannersResponse", false, false),
	op("POST", "/banner/all", "Banners", "List all banners including deleted ones", "", "BannersResponse", false, false),
	op("POST", "/banner/add-new-banner", "Banners", "Create a banner", "BannerCreate", "MessageResponse", true, false),
	op("POST", "/banner/get-banner-by-id/{id}", "Banners", "Get a banner", "", "BannerResponse", false, false),
	op("POST", "/banner/update-banner/{id}", "Banners", "Update a banner", "BannerUpdate", "MessageResponse", true, false),
	op("POST", "/banner/delete-banner/{id}", "Banners", "Delete a banner", "", "MessageResponse", false, false),

	// Blogs
	op("POST", "/blog/get-blog", "Blogs", "List active blogs", "", "BlogsResponse", false, false),
	op("POST", "/blog/add-blog", "Blogs", "Create a blog post", "BlogCreate", "BlogResponse", true, false),
	op("POST", "/blog/get-blog/{id}", "Blogs", "Get a blog post", "", "BlogResponse", false, false),
	op("POST", "/blog/getblogbycategory/{id}", "Blogs", "List blogs by category", "", "BlogsResponse", false, false),
	op("POST", "/blog/update-blog/{id}", "Blogs", "Update a blog post", "BlogUpdate", "BlogResponse", true, false),
	op("POST", "/blog/delete-blog/{id}", "Blogs", "Delete a blog post", "", "MessageResponse", false, false),

	// Blog categories
	op("POST", "/blogcategory/getblogcategory", "Blog Categories", "List blog categories", "", "BlogCategoriesResponse", false, false),
	op("POST", "/blogcategory/addblogcategory", "Blog Categories", "Create a blog category", "NameRequest", "MessageResponse", false, false),
	op("POST", "/blogcategory/getspecificcategory/{id}", "Blog Categories", "Get a blog category", "", "BlogCategoryResponse", false, false),
	op("POST", "/blogcategory/updateblogcategory/{id}", "Blog Categories", "Update a blog category", "BlogCategoryUpdate", "MessageResponse", false, false),
	op("POST", "/blogcategory/deletecategory/{id}", "Blog Categories", "Delete a blog category", "", "MessageResponse", false, false),

	// Content
	op("POST", "/content/getcontent", "Content", "List content blocks", "", "ContentsResponse", false, false),
	op("POST", "/content/createcontent", "Content", "Create a content block", "ContentCreate", "MessageResponse", false, false),
	op("POST", "/content/getspecificcontent/{id}", "Content", "Get a content block", "", "ContentResponse", false, false),
	op("POST", "/content/updatecontent/{id}", "Content", "Update a content block", "ContentUpdate", "MessageResponse", false, false),
	op("POST", "/content/deletecontent/{id}", "Content", "Delete a content block", "", "MessageResponse", false, false),

	// Gallery
	op("POST", "/gallery/getgallerycat", "Gallery", "List active gallery categories", "", "GalleryCategoriesResponse", false, false),
	op("POST", "/gallery/getallgallerycat", "Gallery", "List all gallery categories", "", "GalleryCategoriesResponse", false, false),
	op("POST", "/gallery/creategallery", "Gallery", "Create a gallery category", "GalleryCategoryCreate", "MessageResponse", true, false),
	op("POST", "/gallery/getspecificgallery/{id}", "Gallery", "Get a gallery category", "", "GalleryCategoryResponse", false, false),
	op("POST", "/gallery/updategallery/{id}", "Gallery", "Update a gallery category", "GalleryCategoryUpdate", "MessageResponse", true, false),
	op("POST", "/gallery/deletegallerycat/{id}", "Gallery", "Delete a gallery category", "", "MessageResponse", false, false),

	// Gallery items
	op("POST", "/gallerydetails/getgalleries", "Gallery Items", "List active gallery items", "", "GalleryItemsResponse", false, false),
	op("POST", "/gallerydetails/getallgalleries", "Gallery Items", "List all gallery items", "", "GalleryItemsResponse", false, false),
	op("POST", "/gallerydetails/creategalleryitem", "Gallery Items", "Create a gallery item", "GalleryItemCreate", "MessageResponse", true, false),
	op("POST", "/gallerydetails/getspecificgalleryitem/{id}", "Gallery Items", "Get a gallery item", "", "GalleryItemResponse", false, false),
	op("POST", "/gallerydetails/updategalleryitem/{id}", "Gallery Items", "Update a gallery item", "GalleryItemUpdate", "MessageResponse", true, false),
	op("POST", "/gallerydetails/deletegalleryitem/{id}", "Gallery Items", "Delete a gallery item", "", "MessageResponse", false, false),

	// Contact
	op("POST", "/contact/addcontact", "Contact", "Submit a contact message", "ContactCreate", "MessageResponse", false, false),
	op("POST", "/contact/getcontacts", "Contact", "List contact messages", "", "ContactsResponse", false, false),
	op("POST", "/contact/getcontact/{id}", "Contact", "Get a contact message", "", "ContactResponse", false, false),
	op("POST", "/contact/deletecontact/{id}", "Contact", "Delete a contact message", "", "MessageResponse", false, false),

	// Subscribe
	op("POST", "/subscribe/addsubscribe", "Subscribe", "Subscribe an email address", "EmailRequest", "MessageResponse", false, false),
	op("POST", "/subscribe/getsubscribe", "Subscribe", "List subscribers", "", "SubscribersResponse", false, false),
	op("POST", "/subscribe/deletesubscribe/{id}", "Subscribe", "Delete a subscriber", "", "MessageResponse", false, false),
}

// methodAliases lists the canonical REST methods added alongside the original
// POST-only API. Each alias reuses the request/response schema of the POST
// operation on the same path (or the parent path for /{id} variants) so the
// two forms cannot drift apart.
var methodAliases = []struct {
	method  string
	path    string
	summary string
}{
	// Reads
	{"GET", "/api/getloggedinuser", ""},
	{"GET", "/api/getroles", ""},
	{"GET", "/api/getspecificuser", ""},
	{"GET", "/api/getspecificuser/{id}", ""},
	{"GET", "/api/getusers", ""},
	{"GET", "/api/loggedin", ""},
	{"GET", "/banner/all", ""},
	{"GET", "/banner/get-banner", ""},
	{"GET", "/banner/get-banner-by-id/{id}", ""},
	{"GET", "/blog/get-blog", ""},
	{"GET", "/blog/get-blog/{id}", ""},
	{"GET", "/blog/getblogbycategory/{id}", ""},
	{"GET", "/blogcategory/getblogcategory", ""},
	{"GET", "/blogcategory/getspecificcategory/{id}", ""},
	{"GET", "/category/getcategories", ""},
	{"GET", "/category/getspecificcategory/{id}", ""},
	{"GET", "/category/getsubandsubsub/{id}", ""},
	{"GET", "/color/getcolors", ""},
	{"GET", "/color/getspecificcolor/{id}", ""},
	{"GET", "/contact/getcontact/{id}", ""},
	{"GET", "/contact/getcontacts", ""},
	{"GET", "/content/getcontent", ""},
	{"GET", "/content/getspecificcontent/{id}", ""},
	{"GET", "/coupons/getcoupons", ""},
	{"GET", "/customer/getcart/{id}", ""},
	{"GET", "/customer/getcustomers", ""},
	{"GET", "/customer/getloggedincustomer", ""},
	{"GET", "/customer/getspecificcustomer/{id}", ""},
	{"GET", "/customer/loggedin", ""},
	{"GET", "/dailyrates/getrates", ""},
	{"GET", "/dashboard/getdashboard", ""},
	{"GET", "/dashboard/stats", ""},
	{"GET", "/gallery/getallgallerycat", ""},
	{"GET", "/gallery/getgallerycat", ""},
	{"GET", "/gallery/getspecificgallery/{id}", ""},
	{"GET", "/gallerydetails/getallgalleries", ""},
	{"GET", "/gallerydetails/getgalleries", ""},
	{"GET", "/gallerydetails/getspecificgalleryitem/{id}", ""},
	{"GET", "/gst/getgst", ""},
	{"GET", "/material/getmaterial", ""},
	{"GET", "/orders/getallorders", ""},
	{"GET", "/orders/gethighcustomerdata", ""},
	{"GET", "/orders/getmedcustomerdata", ""},
	{"GET", "/orders/getspecificorder/{id}", ""},
	{"GET", "/orders/gettopsellproducts", ""},
	{"GET", "/product/get-products-for-table", ""},
	{"GET", "/product/getallproducts", ""},
	{"GET", "/product/getproductbytags", ""},
	{"GET", "/product/getproductsbycategoryid/{id}", ""},
	{"GET", "/product/getproductsbysubcategories/{id}", ""},
	{"GET", "/product/getproductsbysubsubcategories/{id}", ""},
	{"GET", "/product/getspecificproduct/{id}", ""},
	{"GET", "/product/getvarproduct", ""},
	{"GET", "/product/getvarproduct/{id}", ""},
	{"GET", "/season/getseasons", ""},
	{"GET", "/size/getsizes", ""},
	{"GET", "/size/getspecificsize/{id}", ""},
	{"GET", "/stocks/getspecificstock/{id}", ""},
	{"GET", "/stocks/getstocks", ""},
	{"GET", "/stocks/getstocksbycategory/{id}", ""},
	{"GET", "/stocks/getstocksbyweighttype/{id}", ""},
	{"GET", "/subcategory/getsubcategories", ""},
	{"GET", "/subcategory/getsubspecificcategory/{id}", ""},
	{"GET", "/subscribe/getsubscribe", ""},
	{"GET", "/subsubcategory/getsubsubcategories", ""},
	{"GET", "/subsubcategory/getsubsubspecificcategory/{id}", ""},

	// Partial updates
	{"PATCH", "/api/updateuser", ""},
	{"PATCH", "/api/updateuser/{id}", ""},
	{"PATCH", "/banner/update-banner/{id}", ""},
	{"PATCH", "/blog/update-blog/{id}", ""},
	{"PATCH", "/blogcategory/updateblogcategory/{id}", ""},
	{"PATCH", "/category/updatecategory/{id}", ""},
	{"PATCH", "/color/updatecolor/{id}", ""},
	{"PATCH", "/content/updatecontent/{id}", ""},
	{"PATCH", "/coupons/updatecoupon/{id}", ""},
	{"PATCH", "/customer/updatecustomer/{id}", ""},
	{"PATCH", "/dailyrates/updaterate/{id}", ""},
	{"PATCH", "/gallery/updategallery/{id}", ""},
	{"PATCH", "/gallerydetails/updategalleryitem/{id}", ""},
	{"PATCH", "/gst/updategst/{id}", ""},
	{"PATCH", "/orders/updateorder/{id}", ""},
	{"PATCH", "/product/updateproduct/{id}", ""},
	{"PATCH", "/size/updatesize/{id}", ""},
	{"PATCH", "/stocks/updatestock/{id}", ""},
	{"PATCH", "/subcategory/updatesubcategory/{id}", ""},
	{"PATCH", "/subsubcategory/updatesubsubcategory/{id}", ""},

	// Complete replacement
	{"PUT", "/category/updatecategory/{id}", "Replace a category"},
	{"PUT", "/customer/updatecustomer/{id}", "Replace a customer profile"},
	{"PUT", "/product/updateproduct/{id}", "Replace a product"},

	// Deletes
	{"DELETE", "/api/deleterole", ""},
	{"DELETE", "/api/deleterole/{id}", ""},
	{"DELETE", "/api/deleteuser", ""},
	{"DELETE", "/api/deleteuser/{id}", ""},
	{"DELETE", "/banner/delete-banner/{id}", ""},
	{"DELETE", "/blog/delete-blog/{id}", ""},
	{"DELETE", "/blogcategory/deletecategory/{id}", ""},
	{"DELETE", "/category/deletecategory/{id}", ""},
	{"DELETE", "/color/deletecolor/{id}", ""},
	{"DELETE", "/contact/deletecontact/{id}", ""},
	{"DELETE", "/content/deletecontent/{id}", ""},
	{"DELETE", "/coupons/deletecoupon/{id}", ""},
	{"DELETE", "/customer/deletecustomer", ""},
	{"DELETE", "/customer/deletecustomer/{id}", ""},
	{"DELETE", "/customer/removefromcart/{id}", ""},
	{"DELETE", "/customer/removefromwishlist/{id}", ""},
	{"DELETE", "/dailyrates/deleterate/{id}", ""},
	{"DELETE", "/gallery/deletegallerycat/{id}", ""},
	{"DELETE", "/gallerydetails/deletegalleryitem/{id}", ""},
	{"DELETE", "/gst/deletegst/{id}", ""},
	{"DELETE", "/orders/deleteorder/{id}", ""},
	{"DELETE", "/product/deleteproduct/{id}", ""},
	{"DELETE", "/size/deletesize/{id}", ""},
	{"DELETE", "/stocks/deletestocks/{id}", ""},
	{"DELETE", "/subcategory/deletesubcategory/{id}", ""},
	{"DELETE", "/subscribe/deletesubscribe/{id}", ""},
	{"DELETE", "/subsubcategory/deletesubsubcategory/{id}", ""},
}

func allOperations() []operation {
	out := make([]operation, 0, len(operations)+len(methodAliases))
	out = append(out, operations...)
	byPath := make(map[string]operation, len(operations))
	for _, o := range operations {
		byPath[o.path] = o
	}
	for _, alias := range methodAliases {
		src, ok := byPath[alias.path]
		if !ok {
			src, ok = byPath[strings.TrimSuffix(alias.path, "/{id}")]
		}
		if !ok {
			src = operation{resp: "MessageResponse"}
		}
		o := src
		o.method = alias.method
		o.path = alias.path
		if alias.summary != "" {
			o.summary = alias.summary
		}
		// Reads and deletes carry their input in the path or query string.
		if alias.method == "GET" || alias.method == "DELETE" {
			o.req = ""
			o.form = false
		}
		out = append(out, o)
	}
	return out
}

func paths() map[string]any {
	ops := allOperations()
	methodsByPath := map[string]map[string]bool{}
	for _, o := range ops {
		if methodsByPath[o.path] == nil {
			methodsByPath[o.path] = map[string]bool{}
		}
		methodsByPath[o.path][o.method] = true
	}

	out := make(map[string]any, len(ops))
	for _, o := range ops {
		pathItem, _ := out[o.path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			out[o.path] = pathItem
		}
		doc := operationDoc(o)
		// A POST is deprecated when a canonical method exists on the same path.
		// POST-only create and action endpoints are not deprecated.
		if o.method == "POST" && len(methodsByPath[o.path]) > 1 {
			doc["deprecated"] = true
			dep := "Deprecated: use " + canonicalMethods(methodsByPath[o.path]) +
				" on this path. The POST form is retained for backward compatibility."
			if desc, _ := doc["description"].(string); desc != "" {
				doc["description"] = desc + "\n\n" + dep
			} else {
				doc["description"] = dep
			}
		}
		pathItem[strings.ToLower(o.method)] = doc
	}
	return out
}

func canonicalMethods(methods map[string]bool) string {
	var names []string
	for _, name := range []string{"GET", "PATCH", "PUT", "DELETE"} {
		if methods[name] {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func operationDoc(o operation) map[string]any {
	if o.resp == "" {
		o.resp = "MessageResponse"
	}

	doc := map[string]any{
		"tags":        []any{o.tag},
		"summary":     o.summary,
		"operationId": operationID(o.method, o.path),
		"responses": map[string]any{
			"200": map[string]any{
				"description": "Successful response",
				"content": map[string]any{
					"application/json": map[string]any{"schema": ref(o.resp)},
				},
			},
			"500": map[string]any{
				"description": "Unexpected server error",
				"content": map[string]any{
					"application/json": map[string]any{"schema": ref("MessageResponse")},
				},
			},
		},
	}

	if o.desc != "" {
		doc["description"] = o.desc
	}
	if o.req != "" {
		contentType := "application/json"
		if o.form {
			contentType = "multipart/form-data"
		}
		doc["requestBody"] = map[string]any{
			"required": true,
			"content": map[string]any{
				contentType: map[string]any{"schema": ref(o.req)},
			},
		}
	}
	if params := pathParameters(o.path); len(params) > 0 {
		doc["parameters"] = params
	}
	if o.auth {
		// Auth is optional at the transport level: a body token also works.
		doc["security"] = []any{
			map[string]any{"bearerAuth": []any{}},
			map[string]any{"cookieAuth": []any{}},
			map[string]any{},
		}
	}

	return doc
}

func pathParameters(path string) []any {
	var params []any
	for _, name := range []string{"id", "resetToken"} {
		if strings.Contains(path, "{"+name+"}") {
			params = append(params, map[string]any{
				"name":        name,
				"in":          "path",
				"required":    true,
				"description": name + " path parameter",
				"schema":      map[string]any{"type": "string"},
			})
		}
	}
	return params
}

func operationID(method, path string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, part := range strings.FieldsFunc(path, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}
