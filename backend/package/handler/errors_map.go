package handler

import (
	"net/http"

	httpErrorHandler "github.com/thesahibnanda-max/eCommerce/backend/package/handler/internal/error_handler"
	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	imagesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/images"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	productsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/products"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
	staticInfoService "github.com/thesahibnanda-max/eCommerce/backend/package/services/static_info"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type ruleGroup struct {
	status int
	rules  []rule
}

type rule struct {
	err     error
	code    string
	message string
}

func defaultRules() []httpErrorHandler.Rule {
	groups := []ruleGroup{
		{http.StatusRequestTimeout, []rule{
			{utils.ErrContextInvalid, "request_canceled", "request was canceled or timed out"},
		}},
		{http.StatusUnauthorized, []rule{
			{usersService.ErrTokenExpired, "token_expired", "token has expired"},
			{usersService.ErrTokenWrongType, "token_wrong_type", "token has the wrong type"},
			{usersService.ErrTokenReused, "token_reused", "refresh token was already used"},
			{usersService.ErrTokenInvalid, "token_invalid", "token is invalid"},
			{usersService.ErrInvalidCredentials, "invalid_credentials", "invalid credentials"},
			{adminService.ErrUnauthorized, "admin_unauthorized", "admin key is invalid"},
			{adminOperationsService.ErrUnauthorized, "admin_unauthorized", "admin key is invalid"},
			{paymentGateway.ErrWebhookSignatureInvalid, "webhook_signature_invalid", "webhook signature is invalid"},
		}},
		{http.StatusForbidden, []rule{
			{paymentGateway.ErrPaymentForbidden, "payment_forbidden", "this payment belongs to someone else"},
		}},
		{http.StatusNotFound, []rule{
			{usersService.ErrUserNotFound, "user_not_found", "user not found"},
			{productsService.ErrProductNotFound, "product_not_found", "product not found"},
			{productsService.ErrVariantNotFound, "variant_not_found", "variant not found"},
			{imagesService.ErrImageNotFound, "image_not_found", "image not found"},
			{cartsService.ErrCartItemNotFound, "cart_item_not_found", "item is not in the cart"},
			{staticInfoService.ErrPageNotFound, "page_not_found", "page not found"},
			{usersRepository.ErrUserNotFound, "user_not_found", "user not found"},
			{addressRepository.ErrAddressNotFound, "address_not_found", "address not found"},
			{ordersRepository.ErrOrderNotFound, "order_not_found", "order not found"},
			{paymentsRepository.ErrPaymentNotFound, "payment_not_found", "payment not found"},
			{paymentsRepository.ErrProofNotFound, "proof_not_found", "this payment has no proof image"},
			{paymentGateway.ErrNotSupported, "not_supported", "not supported by the current payment provider"},
		}},
		{http.StatusConflict, []rule{
			{snapshotService.ErrAlreadyRunning, "snapshot_running", "a snapshot is already running"},
			{usersService.ErrEmailAlreadyExists, "email_already_exists", "email is already registered"},
			{usersService.ErrPhoneAlreadyExists, "phone_number_already_exists", "phone number is already registered"},
			{usersService.ErrEmailAlreadyValidated, "email_already_validated", "email is already validated"},
			{adminService.ErrSKUAlreadyExists, "sku_already_exists", "sku already exists"},
			{adminService.ErrSKUDuplicateInBatch, "sku_duplicate_in_batch", "sku repeated within batch"},
			{adminService.ErrProductDuplicateInBatch, "product_duplicate_in_batch", "product repeated within batch"},
			{adminService.ErrImageDuplicateInBatch, "image_duplicate_in_batch", "image repeated within batch"},
			{imagesService.ErrImageDuplicateInBatch, "image_duplicate_in_batch", "image repeated within batch"},
			{cartsService.ErrOutOfStock, "out_of_stock", "not enough stock"},
			{cartsService.ErrCartFull, "cart_full", "cart has too many items"},
			{ordersService.ErrOutOfStock, "out_of_stock", "not enough stock for an item in your bag"},
			{ordersService.ErrItemUnavailable, "item_unavailable", "an item in your bag is no longer available"},
			{ordersService.ErrOrderCannotPay, "order_cannot_pay", "this order can't be paid right now"},
			{ordersService.ErrOrderCannotCancel, "order_cannot_cancel", "this order can't be cancelled anymore"},
			{ordersService.ErrProofNotAllowed, "proof_not_allowed", "payment proof can't be submitted for this order"},
			{ordersRepository.ErrTransitionNotAllowed, "transition_not_allowed", "that order status change isn't allowed"},
			{ordersRepository.ErrConcurrentUpdate, "order_changed", "the order changed, refresh and try again"},
			{ordersRepository.ErrIdempotencyKeyConflict, "idempotency_key_conflict", "this checkout key was used for a different order"},
			{paymentsRepository.ErrTransitionNotAllowed, "payment_transition_not_allowed", "that payment status change isn't allowed"},
			{paymentsRepository.ErrOpenPaymentConflict, "payment_conflict", "this order already has a payment in progress"},
			{paymentsRepository.ErrProofNotAllowed, "proof_not_allowed", "payment proof can't be submitted right now"},
			{paymentsRepository.ErrProofTransactionReused, "transaction_id_reused", "this transaction id was already used"},
			{paymentGateway.ErrPaymentInProgress, "payment_in_progress", "payment is still being set up, try again shortly"},
			{addressRepository.ErrAddressLimitReached, "address_limit_reached", "you have saved the maximum number of addresses"},
		}},
		{http.StatusRequestEntityTooLarge, []rule{
			{adminService.ErrImageTooLarge, "image_too_large", "image is too large"},
			{adminService.ErrImageTooManyPixels, "image_too_many_pixels", "image has too many pixels"},
			{adminService.ErrBatchTooLarge, "batch_too_large", "batch is too large"},
			{productsService.ErrTooManyIDs, "too_many_ids", "too many ids"},
			{imagesService.ErrTooManyIDs, "too_many_ids", "too many ids"},
			{cartsService.ErrTooManyItems, "too_many_items", "too many items"},
			{paymentsRepository.ErrProofImageTooLarge, "proof_image_too_large", "screenshot must be 5 MB or smaller"},
		}},
		{http.StatusUnsupportedMediaType, []rule{
			{adminService.ErrImageUnsupported, "image_unsupported", "unsupported or corrupt image"},
		}},
		{http.StatusTooManyRequests, []rule{
			{usersService.ErrOTPResendTooSoon, "otp_resend_too_soon", "please wait before requesting another otp"},
			{usersService.ErrOTPTooManyAttempts, "otp_too_many_attempts", "too many invalid attempts, request a new otp"},
		}},
		{http.StatusServiceUnavailable, []rule{
			{adminOperationsService.ErrSnapshotsDisabled, "snapshot_not_configured", "google sheets snapshots are not configured"},
		}},
		{http.StatusBadGateway, []rule{
			{usersService.ErrOTPSendFailed, "otp_send_failed", "could not send otp"},
			{paymentGateway.ErrProviderFailed, "payment_provider_failed", "payment provider is unavailable, try again"},
		}},
		{http.StatusBadRequest, []rule{
			{staticInfoService.ErrSlugInvalid, "slug_invalid", "page slug is invalid"},
			{usersService.ErrOTPInvalid, "otp_invalid", "otp is invalid"},
			{usersService.ErrOTPExpired, "otp_expired", "otp has expired"},
			{usersService.ErrOTPNotFound, "otp_not_found", "no otp was requested"},
			{usersService.ErrOTPRecipientInvalid, "otp_recipient_invalid", "otp recipient is invalid"},
			{usersService.ErrContactRequired, "contact_required", "email or phone number is required"},
			{usersService.ErrNameRequired, "name_required", "name is required"},
			{usersService.ErrEmailRequired, "email_required", "email is required"},
			{usersService.ErrPhoneNumberRequired, "phone_number_required", "phone number is required"},
			{usersService.ErrEmailInvalid, "email_invalid", "email is invalid"},
			{usersService.ErrPhoneNumberInvalid, "phone_number_invalid", "phone number is invalid"},
			{usersService.ErrPasswordInvalid, "password_invalid", "password must be 8+ characters with a letter, a number and a special character"},
			{usersService.ErrUserIDInvalid, "id_invalid", "id is invalid"},
			{adminService.ErrNameRequired, "name_required", "name is required"},
			{adminService.ErrVariantRequired, "variant_required", "at least one variant is required"},
			{adminService.ErrSKURequired, "sku_required", "sku is required"},
			{adminService.ErrSKUDuplicate, "sku_duplicate", "sku repeated within product"},
			{adminService.ErrPriceInvalid, "price_invalid", "price is invalid"},
			{adminService.ErrCurrencyInvalid, "currency_invalid", "currency is invalid"},
			{adminService.ErrProductIDInvalid, "id_invalid", "id is invalid"},
			{adminService.ErrImageIDInvalid, "id_invalid", "id is invalid"},
			{adminService.ErrImageEmpty, "image_empty", "image is empty"},
			{adminService.ErrBatchEmpty, "batch_empty", "batch is empty"},
			{productsService.ErrBatchEmpty, "batch_empty", "ids are required"},
			{productsService.ErrSortInvalid, "sort_invalid", "sort must be relevance, price_asc, price_desc or newest"},
			{productsService.ErrLimitInvalid, "limit_invalid", "limit is invalid"},
			{productsService.ErrOffsetInvalid, "offset_invalid", "offset is invalid"},
			{imagesService.ErrBatchEmpty, "batch_empty", "ids are required"},
			{productsService.ErrIDInvalid, "id_invalid", "id is invalid"},
			{cartsService.ErrBatchEmpty, "batch_empty", "items are required"},
			{cartsService.ErrQuantityInvalid, "quantity_invalid", "quantity is invalid"},
			{cartsService.ErrQuantityTooLarge, "quantity_too_large", "quantity exceeds the per item limit"},
			{cartsService.ErrVariantIDInvalid, "id_invalid", "id is invalid"},
			{ordersService.ErrCartEmpty, "cart_empty", "your bag is empty"},
			{ordersService.ErrTrackingInvalid, "tracking_invalid", "tracking needs a courier, a number and an http(s) link"},
			{ordersService.ErrAddressRequired, "address_required", "choose a delivery address"},
			{ordersService.ErrCurrencyUnsupported, "currency_unsupported", "only INR prices can be checked out"},
			{ordersService.ErrPaymentDecisionInvalid, "decision_invalid", "decision must be SUCCEEDED or FAILED"},
			{ordersRepository.ErrIdempotencyKeyInvalid, "idempotency_key_invalid", "checkout key is invalid"},
			{ordersRepository.ErrOrderIDInvalid, "id_invalid", "order id is invalid"},
			{ordersRepository.ErrStateInvalid, "state_invalid", "order status is invalid"},
			{ordersRepository.ErrCursorInvalid, "cursor_invalid", "cursor is invalid"},
			{ordersRepository.ErrLimitInvalid, "limit_invalid", "limit is invalid"},
			{ordersRepository.ErrReasonTooLong, "reason_too_long", "reason is too long"},
			{paymentsRepository.ErrPaymentIDInvalid, "id_invalid", "payment id is invalid"},
			{paymentsRepository.ErrStateInvalid, "state_invalid", "payment status is invalid"},
			{paymentsRepository.ErrProviderInvalid, "provider_invalid", "payment provider is invalid"},
			{paymentsRepository.ErrCursorInvalid, "cursor_invalid", "cursor is invalid"},
			{paymentsRepository.ErrLimitInvalid, "limit_invalid", "limit is invalid"},
			{paymentsRepository.ErrProofTransactionID, "transaction_id_invalid", "transaction id must be 6 to 35 letters or digits"},
			{paymentsRepository.ErrProofImageInvalid, "proof_image_invalid", "screenshot must be a jpeg, png or webp image"},
			{paymentGateway.ErrPaymentIDInvalid, "id_invalid", "payment id is invalid"},
			{paymentGateway.ErrAmountInvalid, "amount_invalid", "order total is outside what the payment provider accepts"},
			{paymentGateway.ErrStatusInvalid, "decision_invalid", "decision must be SUCCEEDED or FAILED"},
			{paymentGateway.ErrNoteInvalid, "note_invalid", "note must be at most 300 characters"},
			{paymentGateway.ErrWebhookInvalid, "webhook_invalid", "webhook payload is invalid"},
			{paymentGateway.ErrRedirectInvalid, "redirect_invalid", "redirect query is invalid"},
			{usersRepository.ErrIDInvalid, "id_invalid", "id is invalid"},
			{usersRepository.ErrLimitInvalid, "limit_invalid", "limit is invalid"},
			{usersRepository.ErrOffsetInvalid, "offset_invalid", "offset is invalid"},
			{addressRepository.ErrAddressIDInvalid, "id_invalid", "address id is invalid"},
			{addressRepository.ErrFullNameRequired, "full_name_required", "full name is required"},
			{addressRepository.ErrFullNameTooLong, "full_name_too_long", "full name is too long"},
			{addressRepository.ErrPhoneNumberRequired, "phone_number_required", "phone number is required"},
			{addressRepository.ErrPhoneNumberInvalid, "phone_number_invalid", "phone number is invalid"},
			{addressRepository.ErrAddressLine1Required, "address_line1_required", "address line 1 is required"},
			{addressRepository.ErrAddressLine1TooLong, "address_line1_too_long", "address line 1 is too long"},
			{addressRepository.ErrAddressLine2TooLong, "address_line2_too_long", "address line 2 is too long"},
			{addressRepository.ErrLandmarkTooLong, "landmark_too_long", "landmark is too long"},
			{addressRepository.ErrCityTooLong, "city_too_long", "city is too long"},
			{addressRepository.ErrLabelTooLong, "label_too_long", "label is too long"},
			{addressRepository.ErrTextInvalid, "text_invalid", "address contains invalid characters"},
			{addressRepository.ErrPincodeRequired, "pincode_required", "pincode is required"},
			{addressRepository.ErrPincodeInvalid, "pincode_invalid", "pincode must be 6 digits"},
			{addressRepository.ErrPincodeNotFound, "pincode_not_found", "we don't deliver to this pincode yet"},
			{addressRepository.ErrCountryUnsupported, "country_unsupported", "we only deliver within India"},
		}},
	}
	var out []httpErrorHandler.Rule
	for _, g := range groups {
		for _, r := range g.rules {
			out = append(out, httpErrorHandler.Rule{Err: r.err, StatusCode: g.status, Code: r.code, Message: r.message})
		}
	}
	return out
}
