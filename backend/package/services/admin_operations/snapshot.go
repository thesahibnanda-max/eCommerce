package adminOperationsService

import (
	"context"
	"errors"

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

func (s *service) snapshotResponse(status snapshotService.Status, result *snapshotService.Result) SnapshotResponse {
	return SnapshotResponse{
		Configured: true,
		Interval:   humanDuration(s.snapshotInterval),
		Running:    status.Running,
		Last:       status.Last,
		Result:     result,
	}
}
