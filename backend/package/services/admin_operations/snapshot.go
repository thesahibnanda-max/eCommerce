package adminOperationsService

import (
	"context"
	"errors"

	googleAuthService "github.com/thesahibnanda-max/eCommerce/backend/package/services/google_auth"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
)

func (s *service) SnapshotStatus(ctx context.Context, request KeyRequest) (SnapshotResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return SnapshotResponse{}, err
	}
	if s.snapshots == nil {
		return SnapshotResponse{Configured: false}, nil
	}
	return s.snapshotResponse(s.snapshots.Status(), nil), nil
}

func (s *service) RunSnapshot(ctx context.Context, request KeyRequest) (SnapshotResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return SnapshotResponse{}, err
	}
	if s.snapshots == nil {
		return SnapshotResponse{}, ErrSnapshotsDisabled
	}
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.snapshotTimeout)
	defer cancel()
	result, err := s.snapshots.Run(runCtx)
	if err != nil && !errors.Is(err, snapshotService.ErrDatabasesFailed) {
		return SnapshotResponse{}, err
	}
	return s.snapshotResponse(s.snapshots.Status(), &result), nil
}

func (s *service) ConnectGoogle(ctx context.Context, request ConnectGoogleRequest) (ConnectGoogleResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ConnectGoogleResponse{}, err
	}
	if s.google == nil {
		return ConnectGoogleResponse{}, ErrSnapshotsDisabled
	}
	authURL, err := s.google.AuthURL(ctx, googleAuthService.AuthRequest{CallbackURL: request.CallbackURL, ReturnURL: request.ReturnURL})
	if err != nil {
		return ConnectGoogleResponse{}, err
	}
	return ConnectGoogleResponse{AuthURL: authURL}, nil
}

func (s *service) GoogleCallback(ctx context.Context, request GoogleCallbackRequest) (string, error) {
	if s.google == nil {
		return "", ErrSnapshotsDisabled
	}
	return s.google.Callback(ctx, googleAuthService.CallbackRequest{State: request.State, Code: request.Code, Error: request.Error})
}

func (s *service) DisconnectGoogle(ctx context.Context, request KeyRequest) (SnapshotResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return SnapshotResponse{}, err
	}
	if s.google == nil {
		return SnapshotResponse{}, ErrSnapshotsDisabled
	}
	if err := s.google.Disconnect(ctx); err != nil {
		return SnapshotResponse{}, err
	}
	return s.snapshotResponse(s.snapshots.Status(), nil), nil
}

func (s *service) snapshotResponse(status snapshotService.Status, result *snapshotService.Result) SnapshotResponse {
	google := s.google.Status()
	return SnapshotResponse{
		Configured: true,
		Google:     &google,
		Interval:   humanDuration(s.snapshotInterval),
		Running:    status.Running,
		Last:       status.Last,
		Result:     result,
	}
}
