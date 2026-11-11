package docs

var (
	strType  = map[string]any{"type": "string"}
	intType  = map[string]any{"type": "integer"}
	numType  = map[string]any{"type": "number"}
	boolType = map[string]any{"type": "boolean"}
	objType  = map[string]any{"type": "object"}
	anyType  = map[string]any{}
	dateType = map[string]any{"type": "string", "format": "date-time"}
	binType  = map[string]any{"type": "string", "format": "binary"}
)

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func arr(items any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func ref(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func nullable(s map[string]any) map[string]any {
	out := make(map[string]any, len(s)+1)
	for k, v := range s {
		out[k] = v
	}
	out["nullable"] = true
	return out
}

func withID(props map[string]any) map[string]any {
	props["id"] = strType
	props["_id"] = strType
	return props
}

func envelope(key string, schema any) map[string]any {
	return obj(map[string]any{
		"success": boolType,
		"msg":     strType,
		key:       schema,
	})
}

func withDesc(s map[string]any, text string) map[string]any {
	s["description"] = text
	return s
}

func schemas() map[string]any {
	return map[string]any{
		// Shared responses
		"MessageResponse": obj(map[string]any{
			"success": boolType,
			"msg":     strType,
			"message": strType,
			"error":   strType,
		}),
		"SuccessResponse": obj(map[string]any{
			"success": boolType,
		}),

		// Users and roles
		"User": obj(withID(map[string]any{
			"name":      strType,
			"email":     strType,
			"photo":     strType,
			"active":    boolType,
			"createdAt": dateType,
			"updatedAt": dateType,
			"roles":     arr(strType),
		})),
		"UserResponse":  envelope("user", ref("User")),
		"UsersResponse": obj(map[string]any{"users": arr(ref("User"))}),
		"Role": obj(withID(map[string]any{
			"role":        strType,
			"permissions": arr(anyType),
		})),
		"RoleResponse":  envelope("role", ref("Role")),
		"RolesResponse": obj(map[string]any{"roles": arr(ref("Role"))}),
		"LoginUserResponse": obj(map[string]any{
			"success": boolType,
			"msg":     strType,
			"token":   strType,
			"user":    ref("User"),
			"role":    nullable(strType),
		}),
		"UserCreate": obj(map[string]any{
			"name":            strType,
			"email":           strType,
			"password":        strType,
			"confirmPassword": strType,
			"photo":           binType,
			"roles":           arr(strType),
			"active":          boolType,
		}, "name", "email", "password"),
		"UserUpdate": obj(map[string]any{
			"id":     strType,
			"name":   strType,
			"email":  strType,
			"photo":  binType,
			"active": boolType,
			"roles":  arr(strType),
			"token":  strType,
		}),
		"LoginRequest": obj(map[string]any{
			"email":    strType,
			"password": strType,
			"token":    strType,
		}, "email", "password"),
		"RoleCreate": obj(map[string]any{
			"role":        strType,
			"permissions": arr(anyType),
		}, "role"),

		// Categories
		"Category": obj(withID(map[string]any{
			"name":        nullable(strType),
			"image":       nullable(strType),
			"description": nullable(strType),
			"createdAt":   dateType,
			"updatedAt":   dateType,
			"isDeleted":   boolType,
			"isActive":    boolType,
		})),
		"CategoryResponse":   envelope("category", ref("Category")),
		"CategoriesResponse": obj(map[string]any{"categories": arr(ref("Category"))}),
		"CategoryCreate": obj(map[string]any{
			"name":        strType,
			"image":       binType,
			"description": strType,
			"isActive":    boolType,
		}),
		"CategoryUpdate": obj(map[string]any{
			"name":        strType,
			"image":       binType,
			"description": strType,
			"isActive":    boolType,
		}),
		"SubCategory": obj(withID(map[string]any{
			"name":          strType,
			"Category":      nullable(strType),
			"categoryId":    nullable(strType),
			"CategoryTitle": strType,
			"createdAt":     dateType,
			"updatedAt":     dateType,
			"isDeleted":     boolType,
			"isActive":      boolType,
		})),
		"SubCategoryResponse":   envelope("subCategory", ref("SubCategory")),
		"SubCategoriesResponse": obj(map[string]any{"subCategories": arr(ref("SubCategory"))}),
		"SubCategoryCreate": obj(map[string]any{
			"name":       strType,
			"categoryId": strType,
			"isActive":   boolType,
		}, "name"),
		"SubCategoryUpdate": obj(map[string]any{
			"name":       strType,
			"categoryId": strType,
			"isActive":   boolType,
		}),
		"SubSubCategory": obj(withID(map[string]any{
			"name":          strType,
			"Category":      nullable(strType),
			"SubCategory":   nullable(strType),
			"categoryId":    nullable(strType),
			"subCategoryId": nullable(strType),
			"createdAt":     dateType,
			"updatedAt":     dateType,
			"isDeleted":     boolType,
			"isActive":      boolType,
		})),
		"SubSubCategoryResponse":   envelope("subSubCategory", ref("SubSubCategory")),
		"SubSubCategoriesResponse": obj(map[string]any{"subSubCategories": arr(ref("SubSubCategory"))}),
		"SubSubCategoryCreate": obj(map[string]any{
			"name":          strType,
			"categoryId":    strType,
			"subCategoryId": strType,
			"isActive":      boolType,
		}, "name"),
		"SubSubCategoryUpdate": obj(map[string]any{
			"name":          strType,
			"categoryId":    strType,
			"subCategoryId": strType,
			"isActive":      boolType,
		}),
		"SubAndSubSubResponse": obj(map[string]any{
			"subCategories":    arr(ref("SubCategory")),
			"subSubCategories": arr(ref("SubSubCategory")),
		}),

		// Products
		"Product": obj(withID(map[string]any{
			"name":             nullable(strType),
			"description":      nullable(strType),
			"categoryId":       nullable(strType),
			"Category":         nullable(strType),
			"subCategoryId":    nullable(strType),
			"SubCategory":      nullable(strType),
			"subSubCategoryId": nullable(strType),
			"SubSubCategory":   nullable(strType),
			"tags":             arr(anyType),
			"prices": obj(map[string]any{
				"original":        nullable(numType),
				"discounted":      nullable(numType),
				"calculatedPrice": nullable(numType),
			}),
			"pricesOriginal":        nullable(numType),
			"pricesDiscounted":      nullable(numType),
			"pricesCalculatedPrice": nullable(numType),
			"imageGallery":          arr(anyType),
			"stockQuantity":         nullable(intType),
			"hsnCode":               nullable(strType),
			"size":                  nullable(strType),
			"shippingCharge":        nullable(numType),
			"material":              nullable(strType),
			"color":                 nullable(strType),
			"season":                nullable(strType),
			"gstId":                 nullable(strType),
			"sku":                   nullable(strType),
			"calculationOnWeight":   boolType,
			"weightTypeId":          nullable(strType),
			"weight":                nullable(numType),
			"laborCost":             nullable(numType),
			"discountOnLaborCost":   nullable(numType),
			"isActive":              boolType,
			"isProductPopular":      boolType,
			"isProductNew":          boolType,
			"filters":               arr(anyType),
			"isVariant":             boolType,
			"mainProductId":         nullable(strType),
			"productColor":          nullable(strType),
			"productSize":           nullable(strType),
			"otherVariations":       arr(anyType),
			"createdAt":             dateType,
			"updatedAt":             dateType,
		})),
		"ProductResponse":  envelope("product", ref("Product")),
		"ProductsResponse": obj(map[string]any{"products": arr(ref("Product"))}),
		"ProductCreate": obj(map[string]any{
			"name":                  strType,
			"description":           strType,
			"categoryId":            strType,
			"subCategoryId":         strType,
			"subSubCategoryId":      strType,
			"tags":                  arr(anyType),
			"pricesOriginal":        numType,
			"pricesDiscounted":      numType,
			"pricesCalculatedPrice": numType,
			"imageGallery":          arr(binType),
			"stockQuantity":         intType,
			"hsnCode":               strType,
			"size":                  strType,
			"shippingCharge":        numType,
			"material":              strType,
			"color":                 strType,
			"season":                strType,
			"gstId":                 strType,
			"sku":                   strType,
			"calculationOnWeight":   boolType,
			"weightTypeId":          strType,
			"weight":                numType,
			"laborCost":             numType,
			"discountOnLaborCost":   numType,
			"isActive":              boolType,
			"isProductPopular":      boolType,
			"isProductNew":          boolType,
			"filters":               arr(anyType),
			"isVariant":             boolType,
			"mainProductId":         strType,
			"productColor":          strType,
			"productSize":           strType,
			"otherVariations":       arr(anyType),
		}),
		"ProductUpdate": obj(map[string]any{
			"name":                  strType,
			"description":           strType,
			"categoryId":            strType,
			"subCategoryId":         strType,
			"subSubCategoryId":      strType,
			"tags":                  arr(anyType),
			"pricesOriginal":        numType,
			"pricesDiscounted":      numType,
			"pricesCalculatedPrice": numType,
			"imageGallery":          arr(binType),
			"stockQuantity":         intType,
			"isActive":              boolType,
			"gstId":                 strType,
		}),
		"TagRequest": obj(map[string]any{"tag": strType}, "tag"),

		// Customers
		"Customer": obj(withID(map[string]any{
			"username":       strType,
			"email":          strType,
			"address":        anyType,
			"phone":          nullable(strType),
			"paymentMethods": arr(anyType),
			"preferences":    objType,
			"active":         boolType,
			"deleted":        boolType,
			"createdAt":      dateType,
			"updatedAt":      dateType,
			"cartItems":      arr(ref("CartItem")),
			"wishlist":       arr(ref("WishlistItem")),
		})),
		"CustomerResponse":  envelope("customer", ref("Customer")),
		"CustomersResponse": obj(map[string]any{"customers": arr(ref("Customer"))}),
		"CustomerCreate": obj(map[string]any{
			"username":        strType,
			"email":           strType,
			"password":        strType,
			"confirmPassword": strType,
			"phone":           strType,
			"active":          boolType,
		}, "username", "email", "password"),
		"CustomerUpdate": withDesc(obj(map[string]any{
			"customerInfo": obj(map[string]any{
				"username": strType,
				"phone":    nullable(strType),
				"address":  anyType,
			}),
			"username": strType,
			"phone":    strType,
			"address":  anyType,
		}), "Fields may be nested under customerInfo or sent at the top level."),
		"CustomerLoginResponse": obj(map[string]any{
			"success":  boolType,
			"msg":      strType,
			"token":    strType,
			"customer": ref("Customer"),
		}),
		"CartItem": obj(withID(map[string]any{
			"productId": strType,
			"quantity":  intType,
			"tax":       nullable(numType),
			"product":   ref("Product"),
		})),
		"WishlistItem": obj(withID(map[string]any{
			"productId": strType,
			"product":   ref("Product"),
		})),
		"CartAddRequest": obj(map[string]any{
			"productId": strType,
			"quantity":  intType,
		}, "productId"),
		"ProductIDRequest": obj(map[string]any{"productId": strType}, "productId"),
		"CartResponse": obj(map[string]any{
			"success": boolType,
			"cartItems": arr(obj(map[string]any{
				"product":  ref("Product"),
				"quantity": intType,
				"tax":      nullable(numType),
				"stock": obj(map[string]any{
					"id":                  strType,
					"_id":                 strType,
					"quantity":            intType,
					"currentPricePerUnit": numType,
					"ProductId":           strType,
				}),
			})),
		}),

		// Orders
		"Order": obj(withID(map[string]any{
			"customer":             nullable(strType),
			"customerId":           nullable(strType),
			"FirstName":            nullable(strType),
			"LastName":             nullable(strType),
			"status":               strType,
			"totalAmount":          nullable(numType),
			"billingAddress":       nullable(strType),
			"shippingAddress":      nullable(strType),
			"country":              nullable(strType),
			"isInvoiceGenrated":    boolType,
			"invoiceNumber":        nullable(strType),
			"invoiceGenrationDate": nullable(dateType),
			"state":                nullable(strType),
			"city":                 nullable(strType),
			"postCode":             nullable(strType),
			"paymentMethod":        strType,
			"couponCode":           nullable(strType),
			"couponId":             nullable(strType),
			"products":             arr(ref("OrderItem")),
			"createdAt":            dateType,
			"updatedAt":            dateType,
		})),
		"OrderItem": obj(withID(map[string]any{
			"product":     nullable(strType),
			"quantity":    nullable(intType),
			"productData": ref("Product"),
		})),
		"OrderResponse":  envelope("order", ref("Order")),
		"OrdersResponse": obj(map[string]any{"orders": arr(ref("Order"))}),
		"OrderCreate": obj(map[string]any{
			"customerId":      strType,
			"FirstName":       strType,
			"LastName":        strType,
			"status":          strType,
			"totalAmount":     numType,
			"billingAddress":  strType,
			"shippingAddress": strType,
			"country":         strType,
			"invoiceNumber":   strType,
			"state":           strType,
			"city":            strType,
			"postCode":        strType,
			"paymentMethod":   strType,
			"couponId":        strType,
			"products": arr(obj(map[string]any{
				"product":  strType,
				"quantity": intType,
			})),
		}),
		"OrderUpdate": obj(map[string]any{
			"status":            strType,
			"totalAmount":       numType,
			"isInvoiceGenrated": boolType,
		}),
		"ValueCustomersResponse": obj(map[string]any{
			"customers": arr(obj(map[string]any{
				"customerId": strType,
				"total":      numType,
				"customer":   ref("Customer"),
			})),
		}),
		"TopProductsResponse": obj(map[string]any{
			"products": arr(obj(map[string]any{
				"productId": strType,
				"quantity":  intType,
				"product":   ref("Product"),
			})),
		}),
		"DashboardResponse": obj(map[string]any{
			"success": boolType,
			"stats": obj(map[string]any{
				"users":         intType,
				"customers":     intType,
				"products":      intType,
				"orders":        intType,
				"pendingOrders": intType,
				"revenue":       nullable(numType),
			}),
		}),

		// Stocks
		"Stock": obj(withID(map[string]any{
			"ProductId":           nullable(strType),
			"productId":           nullable(strType),
			"name":                nullable(strType),
			"color":               nullable(strType),
			"size":                nullable(strType),
			"quantity":            intType,
			"currentPricePerUnit": numType,
			"date":                nullable(dateType),
		})),
		"StockResponse":  envelope("stock", ref("Stock")),
		"StocksResponse": obj(map[string]any{"stocks": arr(ref("Stock"))}),
		"StockCreate": obj(map[string]any{
			"productId":           strType,
			"name":                strType,
			"color":               strType,
			"size":                strType,
			"quantity":            intType,
			"currentPricePerUnit": numType,
			"date":                dateType,
		}),
		"StockUpdate": obj(map[string]any{
			"quantity":            intType,
			"currentPricePerUnit": numType,
			"name":                strType,
			"color":               strType,
			"size":                strType,
		}),

		// Coupons, GST, daily rates
		"Coupon": obj(withID(map[string]any{
			"name":        strType,
			"type":        strType,
			"description": nullable(strType),
			"start":       nullable(dateType),
			"expiry":      nullable(dateType),
			"discount":    nullable(numType),
			"active":      boolType,
			"createdAt":   dateType,
			"updatedAt":   dateType,
		})),
		"CouponsResponse": obj(map[string]any{"coupons": arr(ref("Coupon"))}),
		"CouponCreate": obj(map[string]any{
			"name":        strType,
			"type":        strType,
			"description": strType,
			"start":       strType,
			"expiry":      strType,
			"discount":    numType,
			"active":      boolType,
		}, "name"),
		"CouponUpdate": obj(map[string]any{
			"name":        strType,
			"type":        strType,
			"description": strType,
			"discount":    numType,
			"active":      boolType,
		}),
		"GST": obj(withID(map[string]any{
			"gst":       numType,
			"sgst":      nullable(numType),
			"cgst":      nullable(numType),
			"igst":      nullable(numType),
			"updatedAt": dateType,
			"isDeleted": boolType,
			"isActive":  boolType,
		})),
		"GSTsResponse": obj(map[string]any{"gsts": arr(ref("GST"))}),
		"GSTCreate": obj(map[string]any{
			"gst":  numType,
			"sgst": numType,
			"cgst": numType,
			"igst": numType,
		}, "gst"),
		"GSTUpdate": obj(map[string]any{
			"gst":  numType,
			"sgst": numType,
			"cgst": numType,
			"igst": numType,
		}),
		"DailyRate": obj(withID(map[string]any{
			"productType": nullable(strType),
			"price":       nullable(numType),
		})),
		"RatesResponse": obj(map[string]any{"rates": arr(ref("DailyRate"))}),
		"RateCreate": obj(map[string]any{
			"productType": strType,
			"price":       numType,
		}),
		"RateUpdate": obj(map[string]any{
			"productType": strType,
			"price":       numType,
		}),

		// Simple masters
		"Color": obj(withID(map[string]any{
			"name":      nullable(strType),
			"createdAt": dateType,
		})),
		"Material": obj(withID(map[string]any{
			"name":      nullable(strType),
			"createdAt": dateType,
		})),
		"Season": obj(withID(map[string]any{
			"name":      nullable(strType),
			"createdAt": dateType,
		})),
		"Size":              obj(withID(map[string]any{"size": nullable(intType)})),
		"ColorsResponse":    obj(map[string]any{"colors": arr(ref("Color"))}),
		"MaterialsResponse": obj(map[string]any{"materials": arr(ref("Material"))}),
		"SeasonsResponse":   obj(map[string]any{"seasons": arr(ref("Season"))}),
		"SizesResponse":     obj(map[string]any{"sizes": arr(ref("Size"))}),
		"ColorResponse":     envelope("color", ref("Color")),
		"SizeResponse":      envelope("size", ref("Size")),
		"NameRequest":       obj(map[string]any{"name": strType}, "name"),
		"ColorCreate": obj(map[string]any{
			"name":  strType,
			"photo": binType,
		}, "name"),
		"SizeCreate": obj(map[string]any{"size": intType}, "size"),

		// Banners
		"Banner": obj(withID(map[string]any{
			"bannerTitle": nullable(strType),
			"image":       nullable(strType),
			"description": nullable(strType),
			"bannerType":  nullable(strType),
			"bannerText":  nullable(strType),
			"createdAt":   dateType,
			"updatedAt":   dateType,
			"isDeleted":   boolType,
			"isActive":    boolType,
		})),
		"BannerResponse":  envelope("banner", ref("Banner")),
		"BannersResponse": obj(map[string]any{"banners": arr(ref("Banner"))}),
		"BannerCreate": obj(map[string]any{
			"bannerTitle": strType,
			"image":       binType,
			"description": strType,
			"bannerType":  strType,
			"bannerText":  strType,
		}),
		"BannerUpdate": obj(map[string]any{
			"bannerTitle": strType,
			"image":       binType,
		}),

		// Blogs
		"Blog": obj(withID(map[string]any{
			"blogFeed":       strType,
			"blogTitle":      strType,
			"imagePath":      strType,
			"description":    nullable(strType),
			"blogCategory":   nullable(strType),
			"blogCategoryId": nullable(strType),
			"date":           dateType,
			"blog":           strType,
			"deleted":        boolType,
			"active":         boolType,
			"createdAt":      dateType,
			"updatedAt":      nullable(dateType),
			"deletedAt":      nullable(dateType),
		})),
		"BlogResponse":  envelope("blog", ref("Blog")),
		"BlogsResponse": obj(map[string]any{"blogs": arr(ref("Blog"))}),
		"BlogCreate": obj(map[string]any{
			"blogFeed":       strType,
			"blogTitle":      strType,
			"imagePath":      binType,
			"description":    strType,
			"blogCategoryId": strType,
			"blog":           strType,
		}, "blogFeed", "blogTitle"),
		"BlogUpdate": obj(map[string]any{
			"blogTitle": strType,
			"blog":      strType,
			"imagePath": binType,
		}),
		"BlogCategory": obj(withID(map[string]any{
			"name":      nullable(strType),
			"active":    boolType,
			"createdAt": dateType,
		})),
		"BlogCategoryResponse":   envelope("category", ref("BlogCategory")),
		"BlogCategoriesResponse": obj(map[string]any{"categories": arr(ref("BlogCategory"))}),
		"BlogCategoryUpdate": obj(map[string]any{
			"name":   strType,
			"active": boolType,
		}),

		// Content
		"Content": obj(withID(map[string]any{
			"contentType": strType,
			"content":     strType,
			"deleted":     boolType,
			"active":      boolType,
			"createdAt":   dateType,
		})),
		"ContentResponse":  envelope("content", ref("Content")),
		"ContentsResponse": obj(map[string]any{"contents": arr(ref("Content"))}),
		"ContentCreate": obj(map[string]any{
			"contentType": strType,
			"content":     strType,
		}, "contentType", "content"),
		"ContentUpdate": obj(map[string]any{
			"contentType": strType,
			"content":     strType,
		}),

		// Gallery
		"GalleryCategory": obj(withID(map[string]any{
			"gallaryCategoryTitle": strType,
			"description":          strType,
			"imagePath":            strType,
			"deleted":              boolType,
			"active":               boolType,
			"createdAt":            dateType,
		})),
		"GalleryCategoryResponse":   envelope("category", ref("GalleryCategory")),
		"GalleryCategoriesResponse": obj(map[string]any{"categories": arr(ref("GalleryCategory"))}),
		"GalleryCategoryCreate": obj(map[string]any{
			"gallaryCategoryTitle": strType,
			"description":          strType,
			"imagePath":            binType,
		}, "gallaryCategoryTitle"),
		"GalleryCategoryUpdate": obj(map[string]any{
			"gallaryCategoryTitle": strType,
			"imagePath":            binType,
		}),
		"GalleryItem": obj(withID(map[string]any{
			"imageTitle":        strType,
			"description":       strType,
			"imagePath":         strType,
			"galleryCategory":   nullable(strType),
			"galleryCategoryId": nullable(strType),
			"deleted":           boolType,
			"active":            boolType,
			"createdAt":         dateType,
		})),
		"GalleryItemResponse":  envelope("item", ref("GalleryItem")),
		"GalleryItemsResponse": obj(map[string]any{"items": arr(ref("GalleryItem"))}),
		"GalleryItemCreate": obj(map[string]any{
			"imageTitle":        strType,
			"description":       strType,
			"imagePath":         binType,
			"galleryCategoryId": strType,
		}, "imageTitle"),
		"GalleryItemUpdate": obj(map[string]any{
			"imageTitle": strType,
			"imagePath":  binType,
		}),

		// Contact and subscribers
		"Contact": obj(withID(map[string]any{
			"name":      nullable(strType),
			"email":     strType,
			"phone":     nullable(strType),
			"subject":   nullable(strType),
			"message":   nullable(strType),
			"createdAt": dateType,
		})),
		"ContactResponse":  envelope("contact", ref("Contact")),
		"ContactsResponse": obj(map[string]any{"contacts": arr(ref("Contact"))}),
		"ContactCreate": obj(map[string]any{
			"name":    strType,
			"email":   strType,
			"phone":   strType,
			"subject": strType,
			"message": strType,
		}, "email"),
		"Subscriber": obj(withID(map[string]any{
			"email":     strType,
			"createdAt": dateType,
		})),
		"SubscribersResponse": obj(map[string]any{"subscribers": arr(ref("Subscriber"))}),

		// Shared request helpers
		"TokenRequest": obj(map[string]any{"token": strType}),
		"IDRequest":    obj(map[string]any{"id": strType}, "id"),
		"EmailRequest": obj(map[string]any{"email": strType}, "email"),
		"PasswordRequest": obj(map[string]any{
			"password": strType,
		}, "password"),
		"UpdatePasswordRequest": obj(map[string]any{
			"id":          strType,
			"oldPassword": strType,
			"newPassword": strType,
			"token":       strType,
		}, "oldPassword", "newPassword"),
	}
}
