package models

import "time"

// Nullable helpers for scanning SQLite rows.

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Photo     string    `json:"photo"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Roles     []string  `json:"roles,omitempty"`
}

type Role struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Permissions string `json:"permissions"`
}

type Customer struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Address        string    `json:"address"`
	Phone          *string   `json:"phone"`
	Password       string    `json:"-"`
	PaymentMethods string    `json:"paymentMethods"`
	Preferences    string    `json:"preferences"`
	Active         bool      `json:"active"`
	Deleted        bool      `json:"deleted"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Product struct {
	ID                    string     `json:"id"`
	Name                  *string    `json:"name"`
	Description           *string    `json:"description"`
	CategoryID            *string    `json:"categoryId"`
	SubCategoryID         *string    `json:"subCategoryId"`
	SubSubCategoryID      *string    `json:"subSubCategoryId"`
	Tags                  string     `json:"tags"`
	PricesOriginal        *float64   `json:"pricesOriginal"`
	PricesDiscounted      *float64   `json:"pricesDiscounted"`
	PricesCalculatedPrice *float64   `json:"pricesCalculatedPrice"`
	ImageGallery          string     `json:"imageGallery"`
	StockQuantity         *int       `json:"stockQuantity"`
	HSNCode               *string    `json:"hsnCode"`
	Size                  *string    `json:"size"`
	ShippingCharge        *float64   `json:"shippingCharge"`
	Material              *string    `json:"material"`
	Color                 *string    `json:"color"`
	Season                *string    `json:"season"`
	GstID                 *string    `json:"gstId"`
	SKU                   *string    `json:"sku"`
	CalculationOnWeight   bool       `json:"calculationOnWeight"`
	WeightTypeID          *string    `json:"weightTypeId"`
	Weight                *float64   `json:"weight"`
	LaborCost             *float64   `json:"laborCost"`
	DiscountOnLaborCost   *float64   `json:"discountOnLaborCost"`
	IsActive              bool       `json:"isActive"`
	IsProductPopular      bool       `json:"isProductPopular"`
	IsProductNew          bool       `json:"isProductNew"`
	Filters               string     `json:"filters"`
	IsVariant             bool       `json:"isVariant"`
	MainProductID         *string    `json:"mainProductId"`
	ProductColor          *string    `json:"productColor"`
	ProductSize           *string    `json:"productSize"`
	OtherVariations       string     `json:"otherVariations"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type Category struct {
	ID          string    `json:"id"`
	Name        *string   `json:"name"`
	Image       *string   `json:"image"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsDeleted   bool      `json:"isDeleted"`
	IsActive    bool      `json:"isActive"`
}

type SubCategory struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CategoryID *string   `json:"categoryId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	IsDeleted  bool      `json:"isDeleted"`
	IsActive   bool      `json:"isActive"`
}

type SubSubCategory struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CategoryID    *string   `json:"categoryId"`
	SubCategoryID *string   `json:"subCategoryId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	IsDeleted     bool      `json:"isDeleted"`
	IsActive      bool      `json:"isActive"`
}

type Order struct {
	ID                   string     `json:"id"`
	CustomerID           *string    `json:"customerId"`
	FirstName            *string    `json:"FirstName"`
	LastName             *string    `json:"LastName"`
	Status               string     `json:"status"`
	TotalAmount          *float64   `json:"totalAmount"`
	BillingAddress       *string    `json:"billingAddress"`
	ShippingAddress      *string    `json:"shippingAddress"`
	Country              *string    `json:"country"`
	IsInvoiceGenerated   bool       `json:"isInvoiceGenrated"`
	InvoiceNumber        *string    `json:"invoiceNumber"`
	InvoiceGenerationDate *time.Time `json:"invoiceGenrationDate"`
	State                *string    `json:"state"`
	City                 *string    `json:"city"`
	PostCode             *string    `json:"postCode"`
	PaymentMethod        string     `json:"paymentMethod"`
	CouponID             *string    `json:"couponId"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type OrderItem struct {
	ID        string  `json:"id"`
	OrderID   string  `json:"orderId"`
	ProductID *string `json:"productId"`
	Quantity  *int    `json:"quantity"`
}

type Stock struct {
	ID                  string     `json:"id"`
	ProductID           *string    `json:"productId"`
	Name                *string    `json:"name"`
	Color               *string    `json:"color"`
	Size                *string    `json:"size"`
	Quantity            int        `json:"quantity"`
	CurrentPricePerUnit float64    `json:"currentPricePerUnit"`
	Date                *time.Time `json:"date"`
}

type Coupon struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description *string    `json:"description"`
	Start       *time.Time `json:"start"`
	Expiry      *time.Time `json:"expiry"`
	Discount    *float64   `json:"discount"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type Gst struct {
	ID        string    `json:"id"`
	Gst       float64   `json:"gst"`
	SGST      *float64  `json:"sgst"`
	CGST      *float64  `json:"cgst"`
	IGST      *float64  `json:"igst"`
	UpdatedAt time.Time `json:"updatedAt"`
	IsDeleted bool      `json:"isDeleted"`
	IsActive  bool      `json:"isActive"`
}

type Banner struct {
	ID          string    `json:"id"`
	BannerTitle *string   `json:"bannerTitle"`
	Image       *string   `json:"image"`
	Description *string   `json:"description"`
	BannerType  *string   `json:"bannerType"`
	BannerText  *string   `json:"bannerText"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsDeleted   bool      `json:"isDeleted"`
	IsActive    bool      `json:"isActive"`
}

type Blog struct {
	ID             string     `json:"id"`
	BlogFeed       string     `json:"blogFeed"`
	BlogTitle      string     `json:"blogTitle"`
	ImagePath      string     `json:"imagePath"`
	Description    *string    `json:"description"`
	BlogCategoryID *string    `json:"blogCategoryId"`
	Date           time.Time  `json:"date"`
	Blog           string     `json:"blog"`
	Deleted        bool       `json:"deleted"`
	Active         bool       `json:"active"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      *time.Time `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt"`
}

type BlogCategory struct {
	ID        string     `json:"id"`
	Name      *string    `json:"name"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt"`
}

type Content struct {
	ID          string     `json:"id"`
	ContentType string     `json:"contentType"`
	Content     string     `json:"content"`
	Deleted     bool       `json:"deleted"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt"`
}

type Contact struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	Subject   *string   `json:"subject"`
	Message   *string   `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type GalleryCategory struct {
	ID                   string     `json:"id"`
	GallaryCategoryTitle string     `json:"gallaryCategoryTitle"`
	Description          string     `json:"description"`
	ImagePath            string     `json:"imagePath"`
	Deleted              bool       `json:"deleted"`
	Active               bool       `json:"active"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            *time.Time `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt"`
}

type GalleryDetails struct {
	ID                string     `json:"id"`
	ImageTitle        string     `json:"imageTitle"`
	Description       string     `json:"description"`
	ImagePath         string     `json:"imagePath"`
	GalleryCategoryID *string    `json:"galleryCategoryId"`
	Deleted           bool       `json:"deleted"`
	Active            bool       `json:"active"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         *time.Time `json:"updatedAt"`
	DeletedAt         *time.Time `json:"deletedAt"`
}

type Color struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Material struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Season struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Size struct {
	ID   string `json:"id"`
	Size *int   `json:"size"`
}

type PriceUpdate struct {
	ID          string   `json:"id"`
	ProductType *string  `json:"productType"`
	Price       *float64 `json:"price"`
}

type Subscribe struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type CartItem struct {
	ID         string   `json:"id"`
	CustomerID string   `json:"customerId"`
	ProductID  string   `json:"productId"`
	Quantity   int      `json:"quantity"`
	Tax        *float64 `json:"tax"`
}

type WishlistItem struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	ProductID  string `json:"productId"`
}
