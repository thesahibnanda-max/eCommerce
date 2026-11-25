package usersService

import (
	"context"
	"errors"
	"strings"

	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func (s *service) isValidated(user usersRepository.Model) bool {
	if s.isOneValidationEnoughFlag {
		return user.IsEmailValidated || user.IsPhoneNumberValidated
	}
	return user.IsEmailValidated && user.IsPhoneNumberValidated
}

func (s *service) issueTokens(user usersRepository.Model) (string, string, error) {
	return s.tokenizer.GenerateNewTokens(map[string]any{payloadKeyID: user.ID})
}

func (s *service) userFromAccessToken(ctx context.Context, accessToken string) (usersRepository.Model, error) {
	payload, err := s.tokenizer.VerifyAccessToken(strings.TrimSpace(accessToken))
	if err != nil {
		return usersRepository.Model{}, err
	}
	id, ok := payload[payloadKeyID].(string)
	if !ok || !utils.ValidateULID(id) {
		return usersRepository.Model{}, ErrTokenInvalid
	}
	return s.usersRepo.GetUserByID(ctx, id)
}

func isOTPRejected(err error) bool {
	return errors.Is(err, otp.ErrOTPInvalid) ||
		errors.Is(err, otp.ErrOTPExpired) ||
		errors.Is(err, otp.ErrOTPNotFound) ||
		errors.Is(err, otp.ErrTooManyAttempts)
}
