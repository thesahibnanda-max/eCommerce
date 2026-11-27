package router

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *routerImpl) registerRoutes() {
	v1 := r.app.Group("/v1")
	{
		users := v1.Group("/users")
		{
			users.Post("/signup", r.handler.SignUp)
			users.Post("/login", r.handler.Login)
			users.Post("/token/refresh", r.handler.RefreshToken)
			users.Post("/email/validation", r.handler.StartEmailValidation)
			users.Post("/email/validate", r.handler.ValidateEmail)
			users.Get("/me", r.handler.GetMe)
		}

		products := v1.Group("/products")
		{
			products.Get("/", r.handler.ListProducts)
			products.Get("/search", r.handler.SearchProducts)
			products.Get("/categories", r.handler.ListCategories)
			products.Get("/categories/:category", r.handler.ListProductsByCategory)
			products.Get("/sku/:sku", r.handler.GetProductBySKU)
			products.Get("/variants/:variantID", r.handler.GetProductByVariantID)
			products.Post("/batch-get", r.handler.GetProductsByIDs)
			products.Get("/:id", r.handler.GetProductByID)
			products.Get("/:id/recommendations", r.handler.RecommendProducts)
		}

		cart := v1.Group("/cart")
		{
			cart.Get("/", r.handler.GetCart)
			cart.Delete("/", r.handler.ClearCart)
			cart.Post("/items", r.handler.AddCartItem)
			cart.Post("/merge", r.handler.MergeCart)
			cart.Put("/items/:variantID", r.handler.SetCartItemQuantity)
			cart.Delete("/items/:variantID", r.handler.RemoveCartItem)
		}

		info := v1.Group("/info")
		{
			info.Get("/pages", r.handler.ListInfoPages)
			info.Get("/pages/:slug", r.handler.GetInfoPage)
		}

		images := v1.Group("/images")
		{
			images.Post("/info/batch", r.handler.GetImagesInfo)
			images.Get("/:id", r.handler.GetImage)
			images.Get("/:id/info", r.handler.GetImageInfo)
		}

		admin := v1.Group("/admin")
		{
			admin.Post("/products", r.handler.AdminCreateProduct)
			admin.Post("/products/batch", r.handler.AdminCreateProducts)
			admin.Put("/products/batch", r.handler.AdminUpdateProducts)
			admin.Put("/products/:id", r.handler.AdminUpdateProduct)
			admin.Post("/images", r.handler.AdminCreateImage)
			admin.Post("/images/batch", r.handler.AdminCreateImages)
			admin.Post("/images/batch-delete", r.handler.AdminDeleteImages)
			admin.Delete("/images/:id", r.handler.AdminDeleteImage)
			admin.Get("/overview", r.handler.AdminOverview)
			admin.Get("/snapshot", r.handler.AdminSnapshotStatus)
			admin.Post("/snapshot", r.handler.AdminRunSnapshot)
			admin.Get("/users", r.handler.AdminListUsers)
			admin.Get("/users/:id", r.handler.AdminGetUser)
			admin.Get("/carts", r.handler.AdminListCarts)
			admin.Get("/products", r.handler.AdminListProducts)
			admin.Get("/images", r.handler.AdminListImages)
			admin.Get("/orders", r.handler.AdminListOrders)
			admin.Get("/orders/:id", r.handler.AdminGetOrder)
			admin.Post("/orders/:id/transition", r.handler.AdminTransitionOrder)
			admin.Post("/orders/:id/tracking", r.handler.AdminSetTracking)
			admin.Get("/payments", r.handler.AdminListPayments)
			admin.Get("/payments/:id", r.handler.AdminGetPayment)
			admin.Get("/payments/:id/proof", r.handler.AdminGetPaymentProof)
			admin.Post("/payments/:id/decision", r.handler.AdminDecidePayment)
		}

		addresses := v1.Group("/addresses")
		{
			addresses.Get("/", r.handler.ListAddresses)
			addresses.Post("/", r.handler.CreateAddress)
			addresses.Put("/:id", r.handler.UpdateAddress)
			addresses.Delete("/:id", r.handler.DeleteAddress)
			addresses.Post("/:id/default", r.handler.SetDefaultAddress)
		}

		v1.Get("/checkout/quote", r.handler.GetCheckoutQuote)

		orders := v1.Group("/orders")
		{
			orders.Get("/", r.handler.ListOrders)
			orders.Post("/", r.handler.PlaceOrder)
			orders.Get("/:id", r.handler.GetOrder)
			orders.Post("/:id/pay", r.handler.PayOrder)
			orders.Post("/:id/payment-proof", r.handler.SubmitPaymentProof)
			orders.Post("/:id/cancel", r.handler.CancelOrder)
		}

		payments := v1.Group("/payments")
		{
			payments.Post("/webhook", r.handler.PaymentWebhook)
			payments.Get("/redirect", r.handler.PaymentRedirect)
		}
	}
}

func panicHandler(c fiber.Ctx, _ any) error {
	return respondError(c, http.StatusInternalServerError, codeInternalError, messageInternalError)
}

func errorHandler(c fiber.Ctx, err error) error {
	if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
		return respondError(c, fiberErr.Code, statusCode(fiberErr.Code), fiberErr.Message)
	}
	return respondError(c, http.StatusInternalServerError, codeInternalError, messageInternalError)
}

func statusCode(status int) string {
	return strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
}

func respondError(c fiber.Ctx, status int, code, message string) error {
	body, err := utils.FromObjectToStringJSON(errorResponse{Code: code, Message: message})
	if err != nil {
		return c.SendStatus(http.StatusInternalServerError)
	}
	return c.Status(status).Type("json").SendString(body)
}

func allowOriginOnPort(port uint) func(origin string) bool {
	want := strconv.FormatUint(uint64(port), 10)
	return func(origin string) bool {
		u, err := url.Parse(origin)
		if err != nil || u.User != nil || u.Hostname() == "" {
			return false
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return false
		}
		if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
		return u.Port() == want
	}
}
