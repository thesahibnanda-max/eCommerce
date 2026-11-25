package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	httpErrorHandler "github.com/thesahibnanda-max/eCommerce/backend/package/handler/internal/error_handler"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	imagesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/images"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	productsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/products"
	staticInfoService "github.com/thesahibnanda-max/eCommerce/backend/package/services/static_info"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
)

const (
	defaultLimit         = 20
	bearerScheme         = "Bearer"
	headerAdminKey       = "X-Admin-Key"
	fieldFile            = "file"
	fieldFiles           = "files"
	fieldMetadata        = "metadata"
	contentTypeAVIF      = "image/avif"
	imageCacheControl    = "public, max-age=31536000, immutable"
	catalogCacheControl  = "public, max-age=60, stale-while-revalidate=300"
	codeInternalError    = "internal_error"
	codeMetadataInvalid  = "metadata_invalid"
	messageInternalError = "internal server error"
	groupAuth            = "auth"
	groupOTP             = "otp"
	groupCatalog         = "catalog"
	groupImages          = "images"
	groupCart            = "cart"
	groupAdmin           = "admin"
	groupInfo            = "info"
	groupOrders          = "orders"
	groupPayments        = "payments"
	fieldTransactionID   = "transaction_id"
)

var rateLimitGroups = []string{groupAuth, groupOTP, groupCatalog, groupImages, groupCart, groupAdmin, groupInfo, groupOrders, groupPayments}

type Handler interface {
	SignUp(c fiber.Ctx) error
	Login(c fiber.Ctx) error
	RefreshToken(c fiber.Ctx) error
	StartEmailValidation(c fiber.Ctx) error
	ValidateEmail(c fiber.Ctx) error
	GetMe(c fiber.Ctx) error

	ListProducts(c fiber.Ctx) error
	SearchProducts(c fiber.Ctx) error
	ListCategories(c fiber.Ctx) error
	ListProductsByCategory(c fiber.Ctx) error
	GetProductBySKU(c fiber.Ctx) error
	GetProductByVariantID(c fiber.Ctx) error
	GetProductsByIDs(c fiber.Ctx) error
	GetProductByID(c fiber.Ctx) error
	RecommendProducts(c fiber.Ctx) error

	GetImage(c fiber.Ctx) error
	GetImageInfo(c fiber.Ctx) error
	GetImagesInfo(c fiber.Ctx) error

	GetCart(c fiber.Ctx) error
	AddCartItem(c fiber.Ctx) error
	SetCartItemQuantity(c fiber.Ctx) error
	RemoveCartItem(c fiber.Ctx) error
	ClearCart(c fiber.Ctx) error
	MergeCart(c fiber.Ctx) error

	ListInfoPages(c fiber.Ctx) error
	GetInfoPage(c fiber.Ctx) error

	AdminCreateProduct(c fiber.Ctx) error
	AdminCreateProducts(c fiber.Ctx) error
	AdminUpdateProduct(c fiber.Ctx) error
	AdminUpdateProducts(c fiber.Ctx) error
	AdminCreateImage(c fiber.Ctx) error
	AdminCreateImages(c fiber.Ctx) error
	AdminDeleteImage(c fiber.Ctx) error
	AdminDeleteImages(c fiber.Ctx) error

	ListAddresses(c fiber.Ctx) error
	CreateAddress(c fiber.Ctx) error
	UpdateAddress(c fiber.Ctx) error
	DeleteAddress(c fiber.Ctx) error
	SetDefaultAddress(c fiber.Ctx) error

	GetCheckoutQuote(c fiber.Ctx) error
	PlaceOrder(c fiber.Ctx) error
	ListOrders(c fiber.Ctx) error
	GetOrder(c fiber.Ctx) error
	PayOrder(c fiber.Ctx) error
	SubmitPaymentProof(c fiber.Ctx) error
	CancelOrder(c fiber.Ctx) error

	PaymentWebhook(c fiber.Ctx) error
	PaymentRedirect(c fiber.Ctx) error

	AdminOverview(c fiber.Ctx) error
	AdminListUsers(c fiber.Ctx) error
	AdminGetUser(c fiber.Ctx) error
	AdminListCarts(c fiber.Ctx) error
	AdminListProducts(c fiber.Ctx) error
	AdminListImages(c fiber.Ctx) error
	AdminListOrders(c fiber.Ctx) error
	AdminGetOrder(c fiber.Ctx) error
	AdminTransitionOrder(c fiber.Ctx) error
	AdminSetTracking(c fiber.Ctx) error
	AdminListPayments(c fiber.Ctx) error
	AdminGetPayment(c fiber.Ctx) error
	AdminGetPaymentProof(c fiber.Ctx) error
	AdminDecidePayment(c fiber.Ctx) error
}

type Services struct {
	Users           usersService.Service
	Admin           adminService.Service
	Products        productsService.Service
	Images          imagesService.Service
	Carts           cartsService.Service
	Info            staticInfoService.Service
	Addresses       addressesService.Service
	Orders          ordersService.Service
	AdminOperations adminOperationsService.Service
}

type handler struct {
	users     usersService.Service
	admin     adminService.Service
	products  productsService.Service
	images    imagesService.Service
	carts     cartsService.Service
	info      staticInfoService.Service
	addresses addressesService.Service
	orders    ordersService.Service
	adminOps  adminOperationsService.Service
	limiter   rateLimiter.Limiter
	errors    httpErrorHandler.ErrorHandler
}

func NewHandler(services Services, limiter rateLimiter.Limiter) (Handler, error) {
	required := []struct {
		missing bool
		err     error
	}{
		{services.Users == nil, ErrUsersServiceRequired},
		{services.Admin == nil, ErrAdminServiceRequired},
		{services.Products == nil, ErrProductsServiceRequired},
		{services.Images == nil, ErrImagesServiceRequired},
		{services.Carts == nil, ErrCartsServiceRequired},
		{services.Info == nil, ErrStaticInfoServiceRequired},
		{services.Addresses == nil, ErrAddressesServiceRequired},
		{services.Orders == nil, ErrOrdersServiceRequired},
		{services.AdminOperations == nil, ErrAdminOpsServiceRequired},
		{limiter == nil, ErrRateLimiterRequired},
	}
	for _, r := range required {
		if r.missing {
			return nil, r.err
		}
	}
	if err := checkRateLimitGroups(limiter); err != nil {
		return nil, err
	}
	return &handler{
		users:     services.Users,
		admin:     services.Admin,
		products:  services.Products,
		images:    services.Images,
		carts:     services.Carts,
		info:      services.Info,
		addresses: services.Addresses,
		orders:    services.Orders,
		adminOps:  services.AdminOperations,
		limiter:   limiter,
		errors:    httpErrorHandler.NewErrorHandler(defaultRules()...),
	}, nil
}

func (h *handler) SignUp(c fiber.Ctx) error {
	if err := h.limit(c, groupAuth); err != nil {
		return h.fail(c, err)
	}
	req, err := decodeBody[usersService.CreateUserRequest](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.CreateUser(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) GetMe(c fiber.Ctx) error {
	token, err := h.authed(c, groupAuth)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.VerifyAccessToken(c.Context(), usersService.VerifyAccessTokenRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) Login(c fiber.Ctx) error {
	if err := h.limit(c, groupAuth); err != nil {
		return h.fail(c, err)
	}
	req, err := decodeBody[usersService.GetUserAndVerifyPasswordRequest](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.GetUserAndVerifyPassword(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) RefreshToken(c fiber.Ctx) error {
	if err := h.limit(c, groupAuth); err != nil {
		return h.fail(c, err)
	}
	req, err := decodeBody[usersService.RefreshAccessTokenRequest](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.RefreshAccessToken(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) StartEmailValidation(c fiber.Ctx) error {
	if err := h.limit(c, groupOTP); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.StartEmailValidation(c.Context(), usersService.StartEmailValidationRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusAccepted, resp)
}

func (h *handler) ValidateEmail(c fiber.Ctx) error {
	if err := h.limit(c, groupOTP); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[ValidateEmailBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.users.ValidateEmail(c.Context(), usersService.ValidateEmailRequest{AccessToken: token, OTP: body.OTP})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) ListProducts(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	offset, limit, err := page(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.ListProducts(c.Context(), productsService.ListProductsRequest{Offset: offset, Limit: limit})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) SearchProducts(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	offset, limit, err := page(c)
	if err != nil {
		return h.fail(c, err)
	}
	minPrice, err := queryInt64Ptr(c, "min_price")
	if err != nil {
		return h.fail(c, err)
	}
	maxPrice, err := queryInt64Ptr(c, "max_price")
	if err != nil {
		return h.fail(c, err)
	}
	inStockOnly, err := queryBool(c, "in_stock_only")
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.SearchProducts(c.Context(), productsService.SearchProductsRequest{
		Text:        c.Query("text"),
		Categories:  queryList(c, "categories"),
		MinPrice:    minPrice,
		MaxPrice:    maxPrice,
		InStockOnly: inStockOnly,
		Sort:        c.Query("sort"),
		Offset:      offset,
		Limit:       limit,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) ListCategories(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.ListCategories(c.Context(), productsService.ListCategoriesRequest{})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) ListProductsByCategory(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	offset, limit, err := page(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.ListProductsByCategory(c.Context(), productsService.ListProductsByCategoryRequest{
		Category: param(c, "category"),
		Offset:   offset,
		Limit:    limit,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) GetProductBySKU(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.GetProductBySKU(c.Context(), productsService.GetProductBySKURequest{SKU: param(c, "sku")})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) GetProductByVariantID(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.GetProductByVariantID(c.Context(), productsService.GetProductByVariantIDRequest{VariantID: param(c, "variantID")})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) GetProductsByIDs(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[IDsBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.GetProductsByIDs(c.Context(), productsService.GetProductsByIDsRequest{IDs: body.IDs})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) GetProductByID(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.GetProductByID(c.Context(), productsService.GetProductByIDRequest{ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) RecommendProducts(c fiber.Ctx) error {
	if err := h.limit(c, groupCatalog); err != nil {
		return h.fail(c, err)
	}
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.products.RecommendProducts(c.Context(), productsService.RecommendProductsRequest{ProductID: param(c, "id"), Limit: limit})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) GetImage(c fiber.Ctx) error {
	if err := h.limit(c, groupImages); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.images.GetImageByID(c.Context(), imagesService.GetImageByIDRequest{ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	contentType := resp.Image.ContentType
	if contentType == "" {
		contentType = contentTypeAVIF
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderCacheControl, imageCacheControl)
	return c.Status(http.StatusOK).Send(resp.Image.Content)
}

func (h *handler) GetImageInfo(c fiber.Ctx) error {
	if err := h.limit(c, groupImages); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.images.GetImageInfoByID(c.Context(), imagesService.GetImageInfoByIDRequest{ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) GetImagesInfo(c fiber.Ctx) error {
	if err := h.limit(c, groupImages); err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[IDsBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.images.GetImagesInfoByIDs(c.Context(), imagesService.GetImagesInfoByIDsRequest{IDs: body.IDs})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) GetCart(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.GetCart(c.Context(), cartsService.GetCartRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AddCartItem(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[cartsService.ItemInput](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.AddItem(c.Context(), cartsService.AddItemRequest{AccessToken: token, VariantID: body.VariantID, Quantity: body.Quantity})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) SetCartItemQuantity(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[QuantityBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.SetItemQuantity(c.Context(), cartsService.SetItemQuantityRequest{AccessToken: token, VariantID: param(c, "variantID"), Quantity: body.Quantity})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) RemoveCartItem(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.RemoveItem(c.Context(), cartsService.RemoveItemRequest{AccessToken: token, VariantID: param(c, "variantID")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) ClearCart(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.ClearCart(c.Context(), cartsService.ClearCartRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) MergeCart(c fiber.Ctx) error {
	if err := h.limit(c, groupCart); err != nil {
		return h.fail(c, err)
	}
	token, err := bearerToken(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[MergeCartBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.carts.MergeItems(c.Context(), cartsService.MergeItemsRequest{AccessToken: token, Items: body.Items})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminCreateProduct(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	product, err := decodeBody[adminService.ProductInput](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.admin.CreateProduct(c.Context(), adminService.CreateProductRequest{AdminKey: key, Product: product})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) AdminCreateProducts(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	req, err := decodeBody[adminService.CreateProductsRequest](c)
	if err != nil {
		return h.fail(c, err)
	}
	req.AdminKey = key
	resp, err := h.admin.CreateProducts(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) AdminUpdateProduct(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	product, err := decodeBody[adminService.ProductInput](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.admin.UpdateProduct(c.Context(), adminService.UpdateProductRequest{
		AdminKey: key,
		Product:  adminService.ProductUpdateInput{ID: param(c, "id"), ProductInput: product},
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminUpdateProducts(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	req, err := decodeBody[adminService.UpdateProductsRequest](c)
	if err != nil {
		return h.fail(c, err)
	}
	req.AdminKey = key
	resp, err := h.admin.UpdateProducts(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminCreateImage(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	fh, err := c.FormFile(fieldFile)
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrFileRequired(fieldFile))
	}
	metadata, err := formMetadata(c)
	if err != nil {
		return h.fail(c, err)
	}
	img, err := readImage(fh, metadata)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.admin.CreateImage(c.Context(), adminService.CreateImageRequest{AdminKey: key, Image: img})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) AdminCreateImages(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	form, err := c.MultipartForm()
	if err != nil || len(form.File[fieldFiles]) == 0 {
		return h.fail(c, httpErrorHandler.ErrFileRequired(fieldFiles))
	}
	images := make([]adminService.ImageInput, len(form.File[fieldFiles]))
	for i, fh := range form.File[fieldFiles] {
		if images[i], err = readImage(fh, nil); err != nil {
			return h.fail(c, err)
		}
	}
	resp, err := h.admin.CreateImages(c.Context(), adminService.CreateImagesRequest{AdminKey: key, Images: images})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) AdminDeleteImage(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.admin.DeleteImage(c.Context(), adminService.DeleteImageRequest{AdminKey: key, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminDeleteImages(c fiber.Ctx) error {
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	key, err := adminKey(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[IDsBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.admin.DeleteImages(c.Context(), adminService.DeleteImagesRequest{AdminKey: key, IDs: body.IDs})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) ListInfoPages(c fiber.Ctx) error {
	if err := h.limit(c, groupInfo); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.info.ListPages(c.Context(), staticInfoService.ListPagesRequest{})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}

func (h *handler) GetInfoPage(c fiber.Ctx) error {
	if err := h.limit(c, groupInfo); err != nil {
		return h.fail(c, err)
	}
	resp, err := h.info.GetPage(c.Context(), staticInfoService.GetPageRequest{Slug: param(c, "slug")})
	if err != nil {
		return h.fail(c, err)
	}
	return respondCached(c, resp)
}
