package usersService

import (
	"context"
	"strings"

	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const payloadKeyID = "id"

type Service interface {
	CreateUser(ctx context.Context, request CreateUserRequest) (CreateUserResponse, error)
	GetUserAndVerifyPassword(ctx context.Context, request GetUserAndVerifyPasswordRequest) (GetUserAndVerifyPasswordResponse, error)
	VerifyAccessToken(ctx context.Context, request VerifyAccessTokenRequest) (VerifyAccessTokenResponse, error)
	RefreshAccessToken(ctx context.Context, request RefreshAccessTokenRequest) (RefreshAccessTokenResponse, error)
	StartEmailValidation(ctx context.Context, request StartEmailValidationRequest) (StartEmailValidationResponse, error)
	ValidateEmail(ctx context.Context, request ValidateEmailRequest) (ValidateEmailResponse, error)
}

type service struct {
	tokenizer                 token.Tokenizer
	usersRepo                 usersRepository.Repository
	emailOTP                  otp.OTP
	isOneValidationEnoughFlag bool
}

func NewService(tokenizer token.Tokenizer, usersRepo usersRepository.Repository, emailOTP otp.OTP, isOneValidationEnough bool) (Service, error) {
	if tokenizer == nil {
		return nil, ErrTokenizerRequired
	}
	if usersRepo == nil {
		return nil, ErrUsersRepoRequired
	}
	if emailOTP == nil {
		return nil, ErrEmailOTPRequired
	}
	return &service{
		tokenizer:                 tokenizer,
		usersRepo:                 usersRepo,
		emailOTP:                  emailOTP,
		isOneValidationEnoughFlag: isOneValidationEnough,
	}, nil
}

func (s *service) CreateUser(ctx context.Context, request CreateUserRequest) (CreateUserResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return CreateUserResponse{}, err
	}

	userResp, err := s.usersRepo.CreateUser(ctx, usersRepository.Model{
		Name:        request.Name,
		Password:    request.Password,
		Email:       request.Email,
		PhoneNumber: request.PhoneNumber,
	})
	if err != nil {
		return CreateUserResponse{}, err
	}

	accessToken, refreshToken, err := s.issueTokens(userResp)
	if err != nil {
		return CreateUserResponse{}, err
	}

	return CreateUserResponse{
		ID:           userResp.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *service) GetUserAndVerifyPassword(ctx context.Context, request GetUserAndVerifyPasswordRequest) (GetUserAndVerifyPasswordResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetUserAndVerifyPasswordResponse{}, err
	}

	var (
		user usersRepository.Model
		err  error
	)
	switch {
	case strings.TrimSpace(request.Email) != "":
		user, err = s.usersRepo.GetUserByEmailAndPassword(ctx, request.Email, request.Password)
	case strings.TrimSpace(request.PhoneNumber) != "":
		user, err = s.usersRepo.GetUserByPhoneNumberAndPassword(ctx, request.PhoneNumber, request.Password)
	default:
		return GetUserAndVerifyPasswordResponse{}, ErrContactRequired
	}
	if err != nil {
		return GetUserAndVerifyPasswordResponse{}, err
	}

	accessToken, refreshToken, err := s.issueTokens(user)
	if err != nil {
		return GetUserAndVerifyPasswordResponse{}, err
	}

	return GetUserAndVerifyPasswordResponse{
		ID:           user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *service) VerifyAccessToken(ctx context.Context, request VerifyAccessTokenRequest) (VerifyAccessTokenResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return VerifyAccessTokenResponse{}, err
	}

	payload, err := s.tokenizer.VerifyAccessToken(strings.TrimSpace(request.AccessToken))
	if err != nil {
		return VerifyAccessTokenResponse{}, err
	}

	id, ok := payload[payloadKeyID]
	if !ok {
		return VerifyAccessTokenResponse{}, ErrTokenInvalid
	}
	idStr, ok := id.(string)
	if !ok || !utils.ValidateULID(idStr) {
		return VerifyAccessTokenResponse{}, ErrTokenInvalid
	}

	user, err := s.usersRepo.GetUserByID(ctx, idStr)
	if err != nil {
		return VerifyAccessTokenResponse{}, err
	}

	return VerifyAccessTokenResponse{
		ID:                     user.ID,
		Name:                   user.Name,
		Email:                  user.Email,
		PhoneNumber:            user.PhoneNumber,
		IsValidated:            s.isValidated(user),
		IsEmailValidated:       user.IsEmailValidated,
		IsPhoneNumberValidated: user.IsPhoneNumberValidated,
	}, nil
}

func (s *service) RefreshAccessToken(ctx context.Context, request RefreshAccessTokenRequest) (RefreshAccessTokenResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return RefreshAccessTokenResponse{}, err
	}

	accessToken, refreshToken, err := s.tokenizer.RefreshAccessToken(strings.TrimSpace(request.RefreshToken))
	if err != nil {
		return RefreshAccessTokenResponse{}, err
	}

	payload, err := s.tokenizer.VerifyAccessToken(accessToken)
	if err != nil {
		return RefreshAccessTokenResponse{}, err
	}
	id, ok := payload[payloadKeyID].(string)
	if !ok || !utils.ValidateULID(id) {
		return RefreshAccessTokenResponse{}, ErrTokenInvalid
	}
	if _, err := s.usersRepo.GetUserByID(ctx, id); err != nil {
		return RefreshAccessTokenResponse{}, err
	}

	return RefreshAccessTokenResponse{
		ID:           id,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *service) StartEmailValidation(ctx context.Context, request StartEmailValidationRequest) (StartEmailValidationResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return StartEmailValidationResponse{}, err
	}
	user, err := s.userFromAccessToken(ctx, request.AccessToken)
	if err != nil {
		return StartEmailValidationResponse{}, err
	}
	if user.IsEmailValidated {
		return StartEmailValidationResponse{}, ErrEmailAlreadyValidated
	}
	if err := s.emailOTP.GenerateAndSendOTP(ctx, user.Email); err != nil {
		return StartEmailValidationResponse{}, err
	}
	return StartEmailValidationResponse{
		ID:    user.ID,
		Email: user.Email,
	}, nil
}

func (s *service) ValidateEmail(ctx context.Context, request ValidateEmailRequest) (ValidateEmailResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ValidateEmailResponse{}, err
	}
	user, err := s.userFromAccessToken(ctx, request.AccessToken)
	if err != nil {
		return ValidateEmailResponse{}, err
	}
	if user.IsEmailValidated {
		return ValidateEmailResponse{ID: user.ID, IsValidated: true}, nil
	}

	ok, err := s.emailOTP.VerifyOTP(ctx, user.Email, request.OTP)
	if isOTPRejected(err) {
		return ValidateEmailResponse{ID: user.ID, IsValidated: false}, err
	}
	if err != nil {
		return ValidateEmailResponse{}, err
	}
	if !ok {
		return ValidateEmailResponse{ID: user.ID, IsValidated: false}, ErrOTPInvalid
	}

	updated, err := s.usersRepo.ValidateEmail(ctx, user.ID)
	if err != nil {
		return ValidateEmailResponse{}, err
	}
	return ValidateEmailResponse{
		ID:          updated.ID,
		IsValidated: updated.IsEmailValidated,
	}, nil
}
