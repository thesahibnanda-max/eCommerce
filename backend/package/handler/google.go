package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
)

const (
	googleCallbackPath   = "/v1/admin/google/callback"
	headerForwardedProto = "X-Forwarded-Proto"
	queryGoogle          = "google"
	queryGoogleReason    = "google_reason"
	googleConnected      = "connected"
	googleFailed         = "error"
	googleNoStore        = "no-store"
	googleNoReferrer     = "no-referrer"
	googleExpiredPage    = "This Google sign-in link expired or was already used. Go back to the admin Backups tab and press Connect Google Drive again."
)

type connectGoogleBody struct {
	ReturnURL string `json:"return_url"`
}

func (h *handler) AdminConnectGoogle(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	body, err := decodeBody[connectGoogleBody](c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.ConnectGoogle(c.Context(), adminOperationsService.ConnectGoogleRequest{
		AdminKey:    key,
		CallbackURL: requestOrigin(c) + googleCallbackPath,
		ReturnURL:   body.ReturnURL,
	})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) AdminGoogleCallback(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, googleNoStore)
	c.Set(fiber.HeaderReferrerPolicy, googleNoReferrer)
	if err := h.limit(c, groupAdmin); err != nil {
		return h.fail(c, err)
	}
	returnURL, err := h.adminOps.GoogleCallback(c.Context(), adminOperationsService.GoogleCallbackRequest{
		State: strings.Clone(c.Query("state")),
		Code:  strings.Clone(c.Query("code")),
		Error: strings.Clone(c.Query("error")),
	})
	if returnURL == "" {
		if err == nil {
			return c.Status(http.StatusBadRequest).SendString(googleExpiredPage)
		}
		if status, _, ok := h.errors.HandleError(err); ok && status < http.StatusInternalServerError {
			return c.Status(status).SendString(googleExpiredPage)
		}
		return h.fail(c, err)
	}
	target, parseErr := url.Parse(returnURL)
	if parseErr != nil {
		return h.fail(c, parseErr)
	}
	query := target.Query()
	query.Set(queryGoogle, googleConnected)
	query.Del(queryGoogleReason)
	if err != nil {
		query.Set(queryGoogle, googleFailed)
		query.Set(queryGoogleReason, h.errorCode(err))
	}
	target.RawQuery = query.Encode()
	return c.Redirect().Status(http.StatusSeeOther).To(target.String())
}

func (h *handler) AdminDisconnectGoogle(c fiber.Ctx) error {
	key, err := h.adminAuthed(c)
	if err != nil {
		return h.fail(c, err)
	}
	resp, err := h.adminOps.DisconnectGoogle(c.Context(), adminOperationsService.KeyRequest{AdminKey: key})
	if err != nil {
		return h.fail(c, err)
	}
	return respond(c, http.StatusOK, resp)
}

func (h *handler) errorCode(err error) string {
	if _, resp, ok := h.errors.HandleError(err); ok {
		return resp.Code
	}
	return codeInternalError
}

func requestOrigin(c fiber.Ctx) string {
	scheme := c.Scheme()
	if forwarded := strings.TrimSpace(strings.Split(c.Get(headerForwardedProto), ",")[0]); forwarded == "https" || forwarded == "http" {
		scheme = forwarded
	}
	return scheme + "://" + c.Host()
}
