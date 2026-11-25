package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	httpErrorHandler "github.com/thesahibnanda-max/eCommerce/backend/package/handler/internal/error_handler"
	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
)

func (h *handler) authed(c fiber.Ctx, group string) (string, error) {
	if err := h.limit(c, group); err != nil {
		return "", err
	}
	return bearerToken(c)
}

func (h *handler) ListAddresses(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.addresses.ListAddresses(c.Context(), addressesService.ListAddressesRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) CreateAddress(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[addressesService.AddressInput](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.addresses.CreateAddress(c.Context(), addressesService.CreateAddressRequest{AccessToken: token, Address: body})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) UpdateAddress(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[addressesService.AddressInput](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.addresses.UpdateAddress(c.Context(), addressesService.UpdateAddressRequest{AccessToken: token, ID: param(c, "id"), Address: body})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) DeleteAddress(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.addresses.DeleteAddress(c.Context(), addressesService.DeleteAddressRequest{AccessToken: token, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) SetDefaultAddress(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.addresses.SetDefaultAddress(c.Context(), addressesService.SetDefaultAddressRequest{AccessToken: token, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) GetCheckoutQuote(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.Quote(c.Context(), ordersService.QuoteRequest{AccessToken: token})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) PlaceOrder(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[PlaceOrderBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.PlaceOrder(c.Context(), ordersService.PlaceOrderRequest{AccessToken: token, AddressID: body.AddressID, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusCreated, resp)
}

func (h *handler) ListOrders(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.ListOrders(c.Context(), ordersService.ListOrdersRequest{
		AccessToken: token,
		States:      queryList(c, "states"),
		Cursor:      strings.Clone(c.Query("cursor")),
		Limit:       limit,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) GetOrder(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.GetOrder(c.Context(), ordersService.GetOrderRequest{AccessToken: token, OrderID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) PayOrder(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.PayOrder(c.Context(), ordersService.PayOrderRequest{AccessToken: token, OrderID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) SubmitPaymentProof(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	fh, err := c.FormFile(fieldFile)
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrFileRequired(fieldFile))
	}
	file, err := fh.Open()
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrFileInvalid(err))
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrFileInvalid(err))
	}
	resp, err := h.orders.SubmitPaymentProof(c.Context(), ordersService.SubmitPaymentProofRequest{
		AccessToken:   token,
		OrderID:       param(c, "id"),
		TransactionID: strings.Clone(c.FormValue(fieldTransactionID)),
		Image:         content,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) CancelOrder(c fiber.Ctx) error {
	token, err := h.authed(c, groupOrders)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.orders.CancelOrder(c.Context(), ordersService.CancelOrderRequest{AccessToken: token, OrderID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) PaymentWebhook(c fiber.Ctx) error {
	if err := h.limit(c, groupPayments); err != nil {
		return h.fail(c, err)
	}
	form, err := url.ParseQuery(string(c.Body()))
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrBodyInvalid(err))
	}
	resp, err := h.orders.HandleProviderWebhook(c.Context(), ordersService.ProviderWebhookRequest{Form: form})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) PaymentRedirect(c fiber.Ctx) error {
	if err := h.limit(c, groupPayments); err != nil {
		return h.fail(c, err)
	}
	query, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return h.fail(c, httpErrorHandler.ErrQueryInvalid("query", err))
	}
	resp, err := h.orders.HandleProviderRedirect(c.Context(), ordersService.ProviderRedirectRequest{Query: query})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}
