package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/docs"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/web"
)

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(a.Cfg.CORSOrigins))

	// Swagger UI + OpenAPI document: /swagger and /swagger/openapi.json
	r.Mount("/swagger", docs.Handler())

	// Static uploads (match Express paths)
	uploads := a.Cfg.UploadsDir
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
		Cfg: a.Cfg,
	})

	// /api – users, roles, payment
	r.Route("/api", func(r chi.Router) {
		r.Post("/register", a.RegisterUser)
		r.Post("/login", a.LoginUser)
		r.Post("/logout", a.LogoutUser)
		r.Post("/getloggedinuser", a.GetLoggedInUser)
		r.Post("/getspecificuser", a.GetSpecificUser)
		r.Post("/loggedin", a.LoginStatus)
		r.Post("/updateuser", a.UpdateUser)
		r.Post("/updatepassword", a.UpdatePassword)
		r.Post("/forgotpassword", a.ForgotPassword)
		r.Post("/resetpassword/{resetToken}", a.ResetPassword)
		r.Post("/getusers", a.GetUsers)
		r.Post("/deleteuser", a.DeleteUser)

		r.Post("/getroles", a.GetRoles)
		r.Post("/createrole", a.CreateRole)
		r.Post("/deleterole", a.DeleteRole)

		r.Post("/payment", a.PaymentGateway)
		r.Post("/create-order", a.PaymentGateway)
	})

	// Categories
	r.Route("/category", func(r chi.Router) {
		r.Post("/addcategory", a.CreateCategory)
		r.Post("/getcategories", a.GetAllCategories)
		r.Post("/getspecificcategory/{id}", a.GetCategoryByID)
		r.Post("/updatecategory/{id}", a.UpdateCategoryByID)
		r.Post("/deletecategory/{id}", a.DeleteCategoryByID)
		r.Post("/getsubandsubsub/{id}", a.GetSubAndSubSubByCategory)
	})

	r.Route("/subcategory", func(r chi.Router) {
		r.Post("/addsubcategory", a.AddSubCategory)
		r.Post("/getsubcategories", a.GetAllSubCategories)
		r.Post("/getsubspecificcategory/{id}", a.GetSubCategoryByID)
		r.Post("/updatesubcategory/{id}", a.UpdateSubCategoryByID)
		r.Post("/deletesubcategory/{id}", a.DeleteSubCategoryByID)
	})

	r.Route("/subsubcategory", func(r chi.Router) {
		r.Post("/addsubsubcategory", a.AddSubSubCategory)
		r.Post("/getsubsubcategories", a.GetAllSubSubCategories)
		r.Post("/getsubsubspecificcategory/{id}", a.GetSubSubCategoryByID)
		r.Post("/updatesubsubcategory/{id}", a.UpdateSubSubCategoryByID)
		r.Post("/deletesubsubcategory/{id}", a.DeleteSubSubCategoryByID)
	})

	// Products
	r.Route("/product", func(r chi.Router) {
		r.Post("/addproduct", a.AddProduct)
		r.Post("/getallproducts", a.GetAllProducts)
		r.Post("/get-products-for-table", a.GetAllProductsForTable)
		r.Post("/addvar", a.AddVarProduct)
		r.Post("/getvarproduct/{id}", a.GetVarProductByID)
		r.Post("/getvarproduct", a.GetAllVarProducts)
		r.Post("/getspecificproduct/{id}", a.GetSpecificProduct)
		r.Post("/updateproduct/{id}", a.UpdateProduct)
		r.Post("/getproductsbycategoryid/{id}", a.GetProductsByCategoryID)
		r.Post("/deleteproduct/{id}", a.DeleteProduct)
		r.Post("/getproductbytags", a.GetProductsByTag)
		r.Post("/getproductsbysubcategories/{id}", a.GetProductsBySubCategoryID)
		r.Post("/getproductsbysubsubcategories/{id}", a.GetProductsBySubSubCategoryID)
	})

	// Customers
	r.Route("/customer", func(r chi.Router) {
		r.Post("/register", a.RegisterCustomer)
		r.Post("/login", a.LoginCustomer)
		r.Post("/logout", a.LogoutCustomer)
		r.Post("/getcustomers", a.GetCustomers)
		r.Post("/getloggedincustomer", a.GetLoggedInCustomer)
		r.Post("/getspecificcustomer/{id}", a.GetSpecificCustomer)
		r.Post("/loggedin", a.CustomerLoginStatus)
		r.Post("/updatecustomer/{id}", a.UpdateCustomer)
		r.Post("/updatepassword/{id}", a.UpdateCustomerPassword)
		r.Post("/forgotpassword", a.ForgotCustomerPassword)
		r.Post("/resetpassword/{resetToken}", a.ResetCustomerPassword)
		r.Post("/deletecustomer", a.DeleteCustomer)
		r.Post("/addtocart/{id}", a.AddToCart)
		r.Post("/getcart/{id}", a.GetCustomerCartItems)
		r.Post("/removefromcart/{id}", a.RemoveFromCart)
		r.Post("/addtowishlist/{id}", a.AddToWishlist)
		r.Post("/removefromwishlist/{id}", a.RemoveFromWishlist)
	})

	// Orders
	r.Route("/orders", func(r chi.Router) {
		r.Post("/addorder", a.CreateOrder)
		r.Post("/getallorders", a.GetOrders)
		r.Post("/getspecificorder/{id}", a.GetOrderByID)
		r.Post("/updateorder/{id}", a.UpdateOrder)
		r.Post("/deleteorder/{id}", a.DeleteOrder)
		r.Post("/gethighcustomerdata", a.GetHighValueCustomers)
		r.Post("/getmedcustomerdata", a.GetMedValueCustomers)
		r.Post("/gettopsellproducts", a.GetTopSellingProducts)
	})

	r.Post("/dashboard/getdashboard", a.Dashboard)
	r.Post("/dashboard/stats", a.Dashboard)

	// Stocks
	r.Route("/stocks", func(r chi.Router) {
		r.Post("/getstocks", a.GetAllStock)
		r.Post("/createstock", a.CreateStock)
		r.Post("/getspecificstock/{id}", a.GetStockByID)
		r.Post("/updatestock/{id}", a.UpdateStockByID)
		r.Post("/deletestocks/{id}", a.DeleteStockByID)
		r.Post("/getstocksbycategory/{id}", a.GetStockByCategory)
		r.Post("/getstocksbyweighttype/{id}", a.GetStockByPriceUpdate)
	})

	// Coupons
	r.Route("/coupons", func(r chi.Router) {
		r.Post("/getcoupons", a.GetCoupons)
		r.Post("/createcoupon", a.CreateCoupon)
		r.Post("/updatecoupon/{id}", a.UpdateCoupon)
		r.Post("/deletecoupon/{id}", a.DeleteCoupon)
	})

	// GST
	r.Route("/gst", func(r chi.Router) {
		r.Post("/getgst", a.GetGSTs)
		r.Post("/creategst", a.CreateGST)
		r.Post("/updategst/{id}", a.UpdateGST)
		r.Post("/deletegst/{id}", a.DeleteGST)
	})

	// Daily rates
	r.Route("/dailyrates", func(r chi.Router) {
		r.Post("/getrates", a.GetDailyRates)
		r.Post("/createrate", a.CreateDailyRate)
		r.Post("/updaterate/{id}", a.UpdateDailyRate)
		r.Post("/deleterate/{id}", a.DeleteDailyRate)
	})

	r.Route("/material", func(r chi.Router) {
		r.Post("/getmaterial", a.GetMaterials)
		r.Post("/creatematerial", a.AddMaterial)
	})
	r.Route("/season", func(r chi.Router) {
		r.Post("/getseasons", a.GetSeasons)
		r.Post("/createseason", a.AddSeason)
	})
	r.Route("/color", func(r chi.Router) {
		r.Post("/getcolors", a.GetColors)
		r.Post("/addcolor", a.AddColor)
		r.Post("/getspecificcolor/{id}", a.GetColorByID)
		r.Post("/updatecolor/{id}", a.UpdateColor)
		r.Post("/deletecolor/{id}", a.DeleteColor)
	})
	r.Route("/size", func(r chi.Router) {
		r.Post("/getsizes", a.GetSizes)
		r.Post("/addsize", a.AddSize)
		r.Post("/getspecificsize/{id}", a.GetSizeByID)
		r.Post("/updatesize/{id}", a.UpdateSize)
		r.Post("/deletesize/{id}", a.DeleteSize)
	})

	// CMS
	r.Route("/banner", func(r chi.Router) {
		r.Post("/get-banner", a.GetBanners)
		r.Post("/all", a.GetAllIncDelBanners)
		r.Post("/add-new-banner", a.AddBanner)
		r.Post("/get-banner-by-id/{id}", a.GetBannerByID)
		r.Post("/update-banner/{id}", a.UpdateBanner)
		r.Post("/delete-banner/{id}", a.DeleteBanner)
	})
	r.Route("/blog", func(r chi.Router) {
		r.Post("/get-blog", a.GetBlogs)
		r.Post("/add-blog", a.AddBlog)
		r.Post("/get-blog/{id}", a.GetBlogByID)
		r.Post("/getblogbycategory/{id}", a.GetBlogByCategory)
		r.Post("/update-blog/{id}", a.UpdateBlog)
		r.Post("/delete-blog/{id}", a.DeleteBlog)
	})
	r.Route("/blogcategory", func(r chi.Router) {
		r.Post("/getblogcategory", a.GetBlogCategories)
		r.Post("/addblogcategory", a.CreateBlogCategory)
		r.Post("/getspecificcategory/{id}", a.GetBlogCategoryByID)
		r.Post("/updateblogcategory/{id}", a.UpdateBlogCategory)
		r.Post("/deletecategory/{id}", a.DeleteBlogCategory)
	})
	r.Route("/content", func(r chi.Router) {
		r.Post("/getcontent", a.GetContents)
		r.Post("/createcontent", a.AddContent)
		r.Post("/getspecificcontent/{id}", a.GetContentByID)
		r.Post("/updatecontent/{id}", a.UpdateContent)
		r.Post("/deletecontent/{id}", a.DeleteContent)
	})
	r.Route("/gallery", func(r chi.Router) {
		r.Post("/getgallerycat", a.GetGalleryCategories)
		r.Post("/getallgallerycat", a.GetAllGalleryCategories)
		r.Post("/creategallery", a.AddGalleryCategory)
		r.Post("/getspecificgallery/{id}", a.GetGalleryCategoryByID)
		r.Post("/updategallery/{id}", a.UpdateGalleryCategory)
		r.Post("/deletegallerycat/{id}", a.DeleteGalleryCategory)
	})
	r.Route("/gallerydetails", func(r chi.Router) {
		r.Post("/getgalleries", a.GetGalleryItems)
		r.Post("/getallgalleries", a.GetAllGalleryItems)
		r.Post("/creategalleryitem", a.AddGalleryItem)
		r.Post("/getspecificgalleryitem/{id}", a.GetGalleryItemByID)
		r.Post("/updategalleryitem/{id}", a.UpdateGalleryItem)
		r.Post("/deletegalleryitem/{id}", a.DeleteGalleryItem)
	})
	r.Route("/contact", func(r chi.Router) {
		r.Post("/addcontact", a.AddContact)
		r.Post("/getcontacts", a.GetContacts)
		r.Post("/getcontact/{id}", a.GetContactByID)
		r.Post("/deletecontact/{id}", a.DeleteContact)
	})
	r.Route("/subscribe", func(r chi.Router) {
		r.Post("/addsubscribe", a.AddSubscribe)
		r.Post("/getsubscribe", a.GetSubscribes)
		r.Post("/deletesubscribe/{id}", a.DeleteSubscribe)
	})

	return r
}
