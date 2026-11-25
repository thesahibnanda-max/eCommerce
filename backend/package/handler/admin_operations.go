package handler

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
)

const proofCacheControl = "private, no-store"

func (h *handler) adminAuthed(c fiber.Ctx) (string, error) {
	if err := h.limit(c, groupAdmin); err != nil {
		return "", err
	}
	return adminKey(c)
}

func (h *handler) adminPage(c fiber.Ctx) (adminOperationsService.PageRequest, error) {
	key, err := h.adminAuthed(c)
	if err != nil {
		return adminOperationsService.PageRequest{}, err
	}
	offset, limit, err := page(c)
	if err != nil {
		return adminOperationsService.PageRequest{}, err
	}
	return adminOperationsService.PageRequest{AdminKey: key, Offset: offset, Limit: limit}, nil
}

func (h *handler) AdminOverview(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.Overview(c.Context(), adminOperationsService.KeyRequest{AdminKey: key})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListUsers(c fiber.Ctx) error {
	req, err := h.adminPage(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListUsers(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminGetUser(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.GetUser(c.Context(), adminOperationsService.IDRequest{AdminKey: key, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListCarts(c fiber.Ctx) error {
	req, err := h.adminPage(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListCarts(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListProducts(c fiber.Ctx) error {
	req, err := h.adminPage(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListProducts(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListImages(c fiber.Ctx) error {
	req, err := h.adminPage(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListImages(c.Context(), req)
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListOrders(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListOrders(c.Context(), adminOperationsService.ListOrdersRequest{
		AdminKey: key,
		States:   queryList(c, "states"),
		Cursor:   strings.Clone(c.Query("cursor")),
		Limit:    limit,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminGetOrder(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.GetOrder(c.Context(), adminOperationsService.IDRequest{AdminKey: key, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminTransitionOrder(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[TransitionOrderBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.TransitionOrder(c.Context(), adminOperationsService.TransitionOrderRequest{AdminKey: key, ID: param(c, "id"), To: body.To, Reason: body.Reason})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminListPayments(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ListPayments(c.Context(), adminOperationsService.ListPaymentsRequest{
		AdminKey: key,
		States:   queryList(c, "states"),
		Provider: strings.Clone(c.Query("provider")),
		Cursor:   strings.Clone(c.Query("cursor")),
		Limit:    limit,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminGetPayment(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.GetPayment(c.Context(), adminOperationsService.IDRequest{AdminKey: key, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminGetPaymentProof(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	proof, err := h.adminOps.GetPaymentProof(c.Context(), adminOperationsService.IDRequest{AdminKey: key, ID: param(c, "id")})
	if err != nil {
		return h.fail(c, err)
	}
	c.Set(fiber.HeaderContentType, proof.ContentType)
	c.Set(fiber.HeaderCacheControl, proofCacheControl)
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.Status(http.StatusOK).Send(proof.Content)
}

func (h *handler) AdminDecidePayment(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[DecidePaymentBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.DecidePayment(c.Context(), adminOperationsService.DecidePaymentRequest{AdminKey: key, ID: param(c, "id"), Status: body.Status, Note: body.Note})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminSetTracking(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[TrackingBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.SetTracking(c.Context(), adminOperationsService.SetTrackingRequest{AdminKey: key, ID: param(c, "id"), Courier: body.Courier, Number: body.Number, URL: body.URL})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}
