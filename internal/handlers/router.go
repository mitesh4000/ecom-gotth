package handlers

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/docs"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/web"
)

// Routes builds the HTTP router. Public, customer, staff and administrator
// scopes are explicit so a forgotten route cannot silently become public.
//
// Every read endpoint is available as GET. Update endpoints are available as
// PATCH (partial) and, for the primary resources, PUT (complete replacement).
// Delete endpoints are available as DELETE. The original POST registration is
// retained on the same path for backward compatibility and is marked
// deprecated in the OpenAPI document. All methods call the same handler and
// service code; there is no duplicate business logic.
func (a *API) Routes() http.Handler {
	cfg := a.Cfg
	if cfg == nil {
		cfg = &config.Config{}
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	if cfg.TrustProxy {
		r.Use(chimw.RealIP)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(middleware.CSRF(cfg.CORSOrigins, cfg.CSRFMode))
	r.Use(middleware.LimitBody(cfg.MaxBodyBytes))

	// Container probes.
	r.Get("/healthz", a.Health)
	r.Get("/readyz", a.Ready)

	// Swagger UI + OpenAPI document: /swagger and /swagger/openapi.json
	r.Mount("/swagger", docs.Handler())

	// Static uploads (match Express paths)
	uploads := cfg.UploadsDir
	fileServer := func(prefix, dir string) {
		r.Handle(prefix+"/*", http.StripPrefix(prefix, http.FileServer(http.Dir(filepath.Join(uploads, dir)))))
	}
	r.Handle("/uploads/*", http.StripPrefix("/uploads", http.FileServer(http.Dir(uploads))))
	fileServer("/gallery-images", "Gallery")
	fileServer("/products", "Products")
	fileServer("/cagtegory", "Category") // intentional typo parity with original
	// Banner/Color share path prefixes with API routes; chi cannot Mount+Route same path.
	// Images remain available under /uploads/Banner and /uploads/Color as well.
	fileServer("/blog-images", "Blog")
	fileServer("/color-images", "Color")
	fileServer("/banner-images", "Banner")

	// Server-rendered pages and embedded static assets. API registration below
	// remains the single source of truth for the existing JSON endpoints.
	web.RegisterWebRoutes(r, web.WebDependencies{
		DB:  a.DB,
		Cfg: cfg,
	})

	strict := cfg.IsStrictAuth()
	userAuth := middleware.ProtectUser(a.DB, cfg.JWTSecret, strict)
	customerAuth := middleware.ProtectCustomer(a.DB, cfg.JWTSecret, strict)
	anyAuth := middleware.ProtectAny(a.DB, cfg.JWTSecret, strict)
	staff := middleware.RequireRoles("admin", "manager", "user")
	manage := middleware.RequireRoles("admin", "manager")
	adminOnly := middleware.RequireRoles("admin")
	authLimiter := middleware.NewRateLimiter(60, time.Minute).Handler()

	// /api – users, roles, payment
	r.Route("/api", func(r chi.Router) {
		// Authentication actions remain POST: they are non-idempotent and are
		// not CRUD operations on a resource.
		r.With(authLimiter).Post("/register", a.RegisterUser)
		r.With(authLimiter).Post("/login", a.LoginUser)
		r.Post("/logout", a.LogoutUser)
		r.With(authLimiter).Post("/forgotpassword", a.ForgotPassword)
		r.With(authLimiter).Post("/resetpassword/{resetToken}", a.ResetPassword)

		// Read endpoints: GET is canonical, POST is a deprecated alias.
		r.Post("/loggedin", a.LoginStatus)
		r.Get("/loggedin", a.LoginStatus)
		r.With(userAuth).Post("/getloggedinuser", a.GetLoggedInUser)
		r.With(userAuth).Get("/getloggedinuser", a.GetLoggedInUser)
		r.With(userAuth).Post("/getspecificuser", a.GetSpecificUser)
		r.With(userAuth).Get("/getspecificuser", a.GetSpecificUser)
		r.With(userAuth).Get("/getspecificuser/{id}", a.GetSpecificUser)
		r.With(userAuth, staff).Post("/getusers", a.GetUsers)
		r.With(userAuth, staff).Get("/getusers", a.GetUsers)

		// Updates are PATCH (partial); POST is a deprecated alias.
		r.With(userAuth).Post("/updateuser", a.UpdateUser)
		r.With(userAuth).Patch("/updateuser", a.UpdateUser)
		r.With(userAuth).Patch("/updateuser/{id}", a.UpdateUser)
		r.With(userAuth).Post("/updatepassword", a.UpdatePassword)

		// Deletes are DELETE; POST is a deprecated alias. The id may be in the
		// path or, for compatibility with the old body-based contract, the query.
		r.With(userAuth, adminOnly).Post("/deleteuser", a.DeleteUser)
		r.With(userAuth, adminOnly).Delete("/deleteuser", a.DeleteUser)
		r.With(userAuth, adminOnly).Delete("/deleteuser/{id}", a.DeleteUser)

		r.With(userAuth, adminOnly).Post("/getroles", a.GetRoles)
		r.With(userAuth, adminOnly).Get("/getroles", a.GetRoles)
		r.With(userAuth, adminOnly).Post("/createrole", a.CreateRole)
		r.With(userAuth, adminOnly).Post("/deleterole", a.DeleteRole)
		r.With(userAuth, adminOnly).Delete("/deleterole", a.DeleteRole)
		r.With(userAuth, adminOnly).Delete("/deleterole/{id}", a.DeleteRole)

		// Payment verification is an action, not a resource CRUD operation.
		r.With(anyAuth).Post("/payment", a.PaymentGateway)
		r.With(anyAuth).Post("/create-order", a.PaymentGateway)
	})

	// Categories
	r.Route("/category", func(r chi.Router) {
		r.Post("/getcategories", a.GetAllCategories)
		r.Get("/getcategories", a.GetAllCategories)
		r.Post("/getspecificcategory/{id}", a.GetCategoryByID)
		r.Get("/getspecificcategory/{id}", a.GetCategoryByID)
		r.Post("/getsubandsubsub/{id}", a.GetSubAndSubSubByCategory)
		r.Get("/getsubandsubsub/{id}", a.GetSubAndSubSubByCategory)
		r.With(userAuth, manage).Post("/addcategory", a.CreateCategory)
		r.With(userAuth, manage).Post("/updatecategory/{id}", a.UpdateCategoryByID)
		r.With(userAuth, manage).Patch("/updatecategory/{id}", a.UpdateCategoryByID)
		r.With(userAuth, manage).Put("/updatecategory/{id}", a.ReplaceCategory)
		r.With(userAuth, manage).Post("/deletecategory/{id}", a.DeleteCategoryByID)
		r.With(userAuth, manage).Delete("/deletecategory/{id}", a.DeleteCategoryByID)
	})

	r.Route("/subcategory", func(r chi.Router) {
		r.Post("/getsubcategories", a.GetAllSubCategories)
		r.Get("/getsubcategories", a.GetAllSubCategories)
		r.Post("/getsubspecificcategory/{id}", a.GetSubCategoryByID)
		r.Get("/getsubspecificcategory/{id}", a.GetSubCategoryByID)
		r.With(userAuth, manage).Post("/addsubcategory", a.AddSubCategory)
		r.With(userAuth, manage).Post("/updatesubcategory/{id}", a.UpdateSubCategoryByID)
		r.With(userAuth, manage).Patch("/updatesubcategory/{id}", a.UpdateSubCategoryByID)
		r.With(userAuth, manage).Post("/deletesubcategory/{id}", a.DeleteSubCategoryByID)
		r.With(userAuth, manage).Delete("/deletesubcategory/{id}", a.DeleteSubCategoryByID)
	})

	r.Route("/subsubcategory", func(r chi.Router) {
		r.Post("/getsubsubcategories", a.GetAllSubSubCategories)
		r.Get("/getsubsubcategories", a.GetAllSubSubCategories)
		r.Post("/getsubsubspecificcategory/{id}", a.GetSubSubCategoryByID)
		r.Get("/getsubsubspecificcategory/{id}", a.GetSubSubCategoryByID)
		r.With(userAuth, manage).Post("/addsubsubcategory", a.AddSubSubCategory)
		r.With(userAuth, manage).Post("/updatesubsubcategory/{id}", a.UpdateSubSubCategoryByID)
		r.With(userAuth, manage).Patch("/updatesubsubcategory/{id}", a.UpdateSubSubCategoryByID)
		r.With(userAuth, manage).Post("/deletesubsubcategory/{id}", a.DeleteSubSubCategoryByID)
		r.With(userAuth, manage).Delete("/deletesubsubcategory/{id}", a.DeleteSubSubCategoryByID)
	})

	// Products
	r.Route("/product", func(r chi.Router) {
		r.Post("/getallproducts", a.GetAllProducts)
		r.Get("/getallproducts", a.GetAllProducts)
		r.Post("/getspecificproduct/{id}", a.GetSpecificProduct)
		r.Get("/getspecificproduct/{id}", a.GetSpecificProduct)
		r.Post("/getproductsbycategoryid/{id}", a.GetProductsByCategoryID)
		r.Get("/getproductsbycategoryid/{id}", a.GetProductsByCategoryID)
		r.Post("/getproductbytags", a.GetProductsByTag)
		r.Get("/getproductbytags", a.GetProductsByTag)
		r.Post("/getproductsbysubcategories/{id}", a.GetProductsBySubCategoryID)
		r.Get("/getproductsbysubcategories/{id}", a.GetProductsBySubCategoryID)
		r.Post("/getproductsbysubsubcategories/{id}", a.GetProductsBySubSubCategoryID)
		r.Get("/getproductsbysubsubcategories/{id}", a.GetProductsBySubSubCategoryID)
		r.Post("/getvarproduct/{id}", a.GetVarProductByID)
		r.Get("/getvarproduct/{id}", a.GetVarProductByID)

		r.With(userAuth, staff).Post("/get-products-for-table", a.GetAllProductsForTable)
		r.With(userAuth, staff).Get("/get-products-for-table", a.GetAllProductsForTable)
		r.With(userAuth, staff).Post("/getvarproduct", a.GetAllVarProducts)
		r.With(userAuth, staff).Get("/getvarproduct", a.GetAllVarProducts)
		r.With(userAuth, manage).Post("/addproduct", a.AddProduct)
		r.With(userAuth, manage).Post("/addvar", a.AddVarProduct)
		r.With(userAuth, manage).Post("/updateproduct/{id}", a.UpdateProduct)
		r.With(userAuth, manage).Patch("/updateproduct/{id}", a.UpdateProduct)
		r.With(userAuth, manage).Put("/updateproduct/{id}", a.ReplaceProduct)
		r.With(userAuth, manage).Post("/deleteproduct/{id}", a.DeleteProduct)
		r.With(userAuth, manage).Delete("/deleteproduct/{id}", a.DeleteProduct)
	})

	// Customers
	r.Route("/customer", func(r chi.Router) {
		r.With(authLimiter).Post("/register", a.RegisterCustomer)
		r.With(authLimiter).Post("/login", a.LoginCustomer)
		r.Post("/logout", a.LogoutCustomer)
		r.Post("/forgotpassword", a.ForgotCustomerPassword)
		r.With(authLimiter).Post("/resetpassword/{resetToken}", a.ResetCustomerPassword)

		r.Post("/loggedin", a.CustomerLoginStatus)
		r.Get("/loggedin", a.CustomerLoginStatus)
		r.With(customerAuth).Post("/getloggedincustomer", a.GetLoggedInCustomer)
		r.With(customerAuth).Get("/getloggedincustomer", a.GetLoggedInCustomer)
		r.With(userAuth, staff).Post("/getcustomers", a.GetCustomers)
		r.With(userAuth, staff).Get("/getcustomers", a.GetCustomers)

		// These handlers perform ownership checks and allow staff access.
		r.With(anyAuth).Post("/getspecificcustomer/{id}", a.GetSpecificCustomer)
		r.With(anyAuth).Get("/getspecificcustomer/{id}", a.GetSpecificCustomer)
		r.With(anyAuth).Post("/updatecustomer/{id}", a.UpdateCustomer)
		r.With(anyAuth).Patch("/updatecustomer/{id}", a.UpdateCustomer)
		r.With(anyAuth).Put("/updatecustomer/{id}", a.ReplaceCustomer)
		r.With(anyAuth).Post("/updatepassword/{id}", a.UpdateCustomerPassword)
		r.With(anyAuth).Post("/addtocart/{id}", a.AddToCart)
		r.With(anyAuth).Post("/getcart/{id}", a.GetCustomerCartItems)
		r.With(anyAuth).Get("/getcart/{id}", a.GetCustomerCartItems)
		r.With(anyAuth).Post("/removefromcart/{id}", a.RemoveFromCart)
		r.With(anyAuth).Delete("/removefromcart/{id}", a.RemoveFromCart)
		r.With(anyAuth).Post("/addtowishlist/{id}", a.AddToWishlist)
		r.With(anyAuth).Post("/removefromwishlist/{id}", a.RemoveFromWishlist)
		r.With(anyAuth).Delete("/removefromwishlist/{id}", a.RemoveFromWishlist)

		r.With(userAuth, manage).Post("/deletecustomer", a.DeleteCustomer)
		r.With(userAuth, manage).Delete("/deletecustomer", a.DeleteCustomer)
		r.With(userAuth, manage).Delete("/deletecustomer/{id}", a.DeleteCustomer)
	})

	// Orders
	r.Route("/orders", func(r chi.Router) {
		r.With(anyAuth).Post("/addorder", a.CreateOrder)
		r.With(anyAuth).Post("/getallorders", a.GetOrders)
		r.With(anyAuth).Get("/getallorders", a.GetOrders)
		r.With(anyAuth).Post("/getspecificorder/{id}", a.GetOrderByID)
		r.With(anyAuth).Get("/getspecificorder/{id}", a.GetOrderByID)
		r.With(userAuth, manage).Post("/updateorder/{id}", a.UpdateOrder)
		r.With(userAuth, manage).Patch("/updateorder/{id}", a.UpdateOrder)
		r.With(userAuth, manage).Post("/deleteorder/{id}", a.DeleteOrder)
		r.With(userAuth, manage).Delete("/deleteorder/{id}", a.DeleteOrder)
		r.With(userAuth, staff).Post("/gethighcustomerdata", a.GetHighValueCustomers)
		r.With(userAuth, staff).Get("/gethighcustomerdata", a.GetHighValueCustomers)
		r.With(userAuth, staff).Post("/getmedcustomerdata", a.GetMedValueCustomers)
		r.With(userAuth, staff).Get("/getmedcustomerdata", a.GetMedValueCustomers)
		r.With(userAuth, staff).Post("/gettopsellproducts", a.GetTopSellingProducts)
		r.With(userAuth, staff).Get("/gettopsellproducts", a.GetTopSellingProducts)
	})

	r.With(userAuth, staff).Post("/dashboard/getdashboard", a.Dashboard)
	r.With(userAuth, staff).Get("/dashboard/getdashboard", a.Dashboard)
	r.With(userAuth, staff).Post("/dashboard/stats", a.Dashboard)
	r.With(userAuth, staff).Get("/dashboard/stats", a.Dashboard)

	// Stocks
	r.Route("/stocks", func(r chi.Router) {
		r.Post("/getstocks", a.GetAllStock)
		r.Get("/getstocks", a.GetAllStock)
		r.Post("/getspecificstock/{id}", a.GetStockByID)
		r.Get("/getspecificstock/{id}", a.GetStockByID)
		r.Post("/getstocksbycategory/{id}", a.GetStockByCategory)
		r.Get("/getstocksbycategory/{id}", a.GetStockByCategory)
		r.Post("/getstocksbyweighttype/{id}", a.GetStockByPriceUpdate)
		r.Get("/getstocksbyweighttype/{id}", a.GetStockByPriceUpdate)
		r.With(userAuth, manage).Post("/createstock", a.CreateStock)
		r.With(userAuth, manage).Post("/updatestock/{id}", a.UpdateStockByID)
		r.With(userAuth, manage).Patch("/updatestock/{id}", a.UpdateStockByID)
		r.With(userAuth, manage).Post("/deletestocks/{id}", a.DeleteStockByID)
		r.With(userAuth, manage).Delete("/deletestocks/{id}", a.DeleteStockByID)
	})

	// Coupons
	r.Route("/coupons", func(r chi.Router) {
		r.Post("/getcoupons", a.GetCoupons)
		r.Get("/getcoupons", a.GetCoupons)
		r.With(userAuth, manage).Post("/createcoupon", a.CreateCoupon)
		r.With(userAuth, manage).Post("/updatecoupon/{id}", a.UpdateCoupon)
		r.With(userAuth, manage).Patch("/updatecoupon/{id}", a.UpdateCoupon)
		r.With(userAuth, manage).Post("/deletecoupon/{id}", a.DeleteCoupon)
		r.With(userAuth, manage).Delete("/deletecoupon/{id}", a.DeleteCoupon)
	})

	// GST
	r.Route("/gst", func(r chi.Router) {
		r.Post("/getgst", a.GetGSTs)
		r.Get("/getgst", a.GetGSTs)
		r.With(userAuth, manage).Post("/creategst", a.CreateGST)
		r.With(userAuth, manage).Post("/updategst/{id}", a.UpdateGST)
		r.With(userAuth, manage).Patch("/updategst/{id}", a.UpdateGST)
		r.With(userAuth, manage).Post("/deletegst/{id}", a.DeleteGST)
		r.With(userAuth, manage).Delete("/deletegst/{id}", a.DeleteGST)
	})

	// Daily rates
	r.Route("/dailyrates", func(r chi.Router) {
		r.Post("/getrates", a.GetDailyRates)
		r.Get("/getrates", a.GetDailyRates)
		r.With(userAuth, manage).Post("/createrate", a.CreateDailyRate)
		r.With(userAuth, manage).Post("/updaterate/{id}", a.UpdateDailyRate)
		r.With(userAuth, manage).Patch("/updaterate/{id}", a.UpdateDailyRate)
		r.With(userAuth, manage).Post("/deleterate/{id}", a.DeleteDailyRate)
		r.With(userAuth, manage).Delete("/deleterate/{id}", a.DeleteDailyRate)
	})

	r.Route("/material", func(r chi.Router) {
		r.Post("/getmaterial", a.GetMaterials)
		r.Get("/getmaterial", a.GetMaterials)
		r.With(userAuth, manage).Post("/creatematerial", a.AddMaterial)
	})
	r.Route("/season", func(r chi.Router) {
		r.Post("/getseasons", a.GetSeasons)
		r.Get("/getseasons", a.GetSeasons)
		r.With(userAuth, manage).Post("/createseason", a.AddSeason)
	})
	r.Route("/color", func(r chi.Router) {
		r.Post("/getcolors", a.GetColors)
		r.Get("/getcolors", a.GetColors)
		r.Post("/getspecificcolor/{id}", a.GetColorByID)
		r.Get("/getspecificcolor/{id}", a.GetColorByID)
		r.With(userAuth, manage).Post("/addcolor", a.AddColor)
		r.With(userAuth, manage).Post("/updatecolor/{id}", a.UpdateColor)
		r.With(userAuth, manage).Patch("/updatecolor/{id}", a.UpdateColor)
		r.With(userAuth, manage).Post("/deletecolor/{id}", a.DeleteColor)
		r.With(userAuth, manage).Delete("/deletecolor/{id}", a.DeleteColor)
	})
	r.Route("/size", func(r chi.Router) {
		r.Post("/getsizes", a.GetSizes)
		r.Get("/getsizes", a.GetSizes)
		r.Post("/getspecificsize/{id}", a.GetSizeByID)
		r.Get("/getspecificsize/{id}", a.GetSizeByID)
		r.With(userAuth, manage).Post("/addsize", a.AddSize)
		r.With(userAuth, manage).Post("/updatesize/{id}", a.UpdateSize)
		r.With(userAuth, manage).Patch("/updatesize/{id}", a.UpdateSize)
		r.With(userAuth, manage).Post("/deletesize/{id}", a.DeleteSize)
		r.With(userAuth, manage).Delete("/deletesize/{id}", a.DeleteSize)
	})

	// CMS
	r.Route("/banner", func(r chi.Router) {
		r.Post("/get-banner", a.GetBanners)
		r.Get("/get-banner", a.GetBanners)
		r.Post("/get-banner-by-id/{id}", a.GetBannerByID)
		r.Get("/get-banner-by-id/{id}", a.GetBannerByID)
		r.With(userAuth, staff).Post("/all", a.GetAllIncDelBanners)
		r.With(userAuth, staff).Get("/all", a.GetAllIncDelBanners)
		r.With(userAuth, manage).Post("/add-new-banner", a.AddBanner)
		r.With(userAuth, manage).Post("/update-banner/{id}", a.UpdateBanner)
		r.With(userAuth, manage).Patch("/update-banner/{id}", a.UpdateBanner)
		r.With(userAuth, manage).Post("/delete-banner/{id}", a.DeleteBanner)
		r.With(userAuth, manage).Delete("/delete-banner/{id}", a.DeleteBanner)
	})
	r.Route("/blog", func(r chi.Router) {
		r.Post("/get-blog", a.GetBlogs)
		r.Get("/get-blog", a.GetBlogs)
		r.Post("/get-blog/{id}", a.GetBlogByID)
		r.Get("/get-blog/{id}", a.GetBlogByID)
		r.Post("/getblogbycategory/{id}", a.GetBlogByCategory)
		r.Get("/getblogbycategory/{id}", a.GetBlogByCategory)
		r.With(userAuth, manage).Post("/add-blog", a.AddBlog)
		r.With(userAuth, manage).Post("/update-blog/{id}", a.UpdateBlog)
		r.With(userAuth, manage).Patch("/update-blog/{id}", a.UpdateBlog)
		r.With(userAuth, manage).Post("/delete-blog/{id}", a.DeleteBlog)
		r.With(userAuth, manage).Delete("/delete-blog/{id}", a.DeleteBlog)
	})
	r.Route("/blogcategory", func(r chi.Router) {
		r.Post("/getblogcategory", a.GetBlogCategories)
		r.Get("/getblogcategory", a.GetBlogCategories)
		r.Post("/getspecificcategory/{id}", a.GetBlogCategoryByID)
		r.Get("/getspecificcategory/{id}", a.GetBlogCategoryByID)
		r.With(userAuth, manage).Post("/addblogcategory", a.CreateBlogCategory)
		r.With(userAuth, manage).Post("/updateblogcategory/{id}", a.UpdateBlogCategory)
		r.With(userAuth, manage).Patch("/updateblogcategory/{id}", a.UpdateBlogCategory)
		r.With(userAuth, manage).Post("/deletecategory/{id}", a.DeleteBlogCategory)
		r.With(userAuth, manage).Delete("/deletecategory/{id}", a.DeleteBlogCategory)
	})
	r.Route("/content", func(r chi.Router) {
		r.Post("/getcontent", a.GetContents)
		r.Get("/getcontent", a.GetContents)
		r.Post("/getspecificcontent/{id}", a.GetContentByID)
		r.Get("/getspecificcontent/{id}", a.GetContentByID)
		r.With(userAuth, manage).Post("/createcontent", a.AddContent)
		r.With(userAuth, manage).Post("/updatecontent/{id}", a.UpdateContent)
		r.With(userAuth, manage).Patch("/updatecontent/{id}", a.UpdateContent)
		r.With(userAuth, manage).Post("/deletecontent/{id}", a.DeleteContent)
		r.With(userAuth, manage).Delete("/deletecontent/{id}", a.DeleteContent)
	})
	r.Route("/gallery", func(r chi.Router) {
		r.Post("/getgallerycat", a.GetGalleryCategories)
		r.Get("/getgallerycat", a.GetGalleryCategories)
		r.Post("/getspecificgallery/{id}", a.GetGalleryCategoryByID)
		r.Get("/getspecificgallery/{id}", a.GetGalleryCategoryByID)
		r.With(userAuth, staff).Post("/getallgallerycat", a.GetAllGalleryCategories)
		r.With(userAuth, staff).Get("/getallgallerycat", a.GetAllGalleryCategories)
		r.With(userAuth, manage).Post("/creategallery", a.AddGalleryCategory)
		r.With(userAuth, manage).Post("/updategallery/{id}", a.UpdateGalleryCategory)
		r.With(userAuth, manage).Patch("/updategallery/{id}", a.UpdateGalleryCategory)
		r.With(userAuth, manage).Post("/deletegallerycat/{id}", a.DeleteGalleryCategory)
		r.With(userAuth, manage).Delete("/deletegallerycat/{id}", a.DeleteGalleryCategory)
	})
	r.Route("/gallerydetails", func(r chi.Router) {
		r.Post("/getgalleries", a.GetGalleryItems)
		r.Get("/getgalleries", a.GetGalleryItems)
		r.Post("/getspecificgalleryitem/{id}", a.GetGalleryItemByID)
		r.Get("/getspecificgalleryitem/{id}", a.GetGalleryItemByID)
		r.With(userAuth, staff).Post("/getallgalleries", a.GetAllGalleryItems)
		r.With(userAuth, staff).Get("/getallgalleries", a.GetAllGalleryItems)
		r.With(userAuth, manage).Post("/creategalleryitem", a.AddGalleryItem)
		r.With(userAuth, manage).Post("/updategalleryitem/{id}", a.UpdateGalleryItem)
		r.With(userAuth, manage).Patch("/updategalleryitem/{id}", a.UpdateGalleryItem)
		r.With(userAuth, manage).Post("/deletegalleryitem/{id}", a.DeleteGalleryItem)
		r.With(userAuth, manage).Delete("/deletegalleryitem/{id}", a.DeleteGalleryItem)
	})
	r.Route("/contact", func(r chi.Router) {
		r.Post("/addcontact", a.AddContact)
		r.With(userAuth, staff).Post("/getcontacts", a.GetContacts)
		r.With(userAuth, staff).Get("/getcontacts", a.GetContacts)
		r.With(userAuth, staff).Post("/getcontact/{id}", a.GetContactByID)
		r.With(userAuth, staff).Get("/getcontact/{id}", a.GetContactByID)
		r.With(userAuth, manage).Post("/deletecontact/{id}", a.DeleteContact)
		r.With(userAuth, manage).Delete("/deletecontact/{id}", a.DeleteContact)
	})
	r.Route("/subscribe", func(r chi.Router) {
		r.Post("/addsubscribe", a.AddSubscribe)
		r.With(userAuth, staff).Post("/getsubscribe", a.GetSubscribes)
		r.With(userAuth, staff).Get("/getsubscribe", a.GetSubscribes)
		r.With(userAuth, manage).Post("/deletesubscribe/{id}", a.DeleteSubscribe)
		r.With(userAuth, manage).Delete("/deletesubscribe/{id}", a.DeleteSubscribe)
	})

	return r
}
