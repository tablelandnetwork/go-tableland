package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemMockService is a dummy implementation that returns a fixed value.
type SystemMockService struct {
}

// NewSystemMockService creates a new SystemMockService.
func NewSystemMockService() system.SystemService {
	return &SystemMockService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockService) GetTableMetadata(ctx context.Context, uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{
		ExternalURL: fmt.Sprintf("https://tableland.com/tables/%s", uuid.String()),
		Image:       "https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png", //nolint
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       1546360800,
			},
		},
	}, nil
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *SystemMockService) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{}, nil
}

// Authorize authorizes an address in the SQLStore.
func (s *SystemMockService) Authorize(ctx context.Context, address string) error {
	return nil
}

// Revoke removes an address' access in the SQLStore.
func (s *SystemMockService) Revoke(ctx context.Context, address string) error {
	return nil
}

// IsAuthorized checks the authorization status of an address in the SQLStore.
func (s *SystemMockService) IsAuthorized(ctx context.Context, address string) (sqlstore.IsAuthorizedResult, error) {
	return sqlstore.IsAuthorizedResult{IsAuthorized: true}, nil
}

// GetAuthorizationRecord gets the authorization record for the provided address from the SQLStore.
func (s *SystemMockService) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	return sqlstore.AuthorizationRecord{Address: "some-address"}, nil
}

// ListAuthorized lists all authorization records in the SQLStore.
func (s *SystemMockService) ListAuthorized(ctx context.Context) ([]sqlstore.AuthorizationRecord, error) {
	return []sqlstore.AuthorizationRecord{}, nil
}

// SystemMockErrService is a dummy implementation that returns a fixed value.
type SystemMockErrService struct {
}

// NewSystemMockErrService creates a new SystemMockErrService.
func NewSystemMockErrService() system.SystemService {
	return &SystemMockErrService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockErrService) GetTableMetadata(ctx context.Context, uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{}, errors.New("table not found")
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *SystemMockErrService) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{}, errors.New("no table found")
}

// Authorize authorizes an address in the SQLStore.
func (s *SystemMockErrService) Authorize(ctx context.Context, address string) error {
	return errors.New("error authorizing")
}

// Revoke removes an address' access in the SQLStore.
func (s *SystemMockErrService) Revoke(ctx context.Context, address string) error {
	return errors.New("error revoking")
}

// IsAuthorized checks the authorization status of an address in the SQLStore.
func (s *SystemMockErrService) IsAuthorized(ctx context.Context, address string) (sqlstore.IsAuthorizedResult, error) {
	return sqlstore.IsAuthorizedResult{}, errors.New("error checking authorization")
}

// GetAuthorizationRecord gets the authorization record for the provided address from the SQLStore.
func (s *SystemMockErrService) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	return sqlstore.AuthorizationRecord{}, errors.New("error getting authorization record")
}

// ListAuthorized lists all authorization records in the SQLStore.
func (s *SystemMockErrService) ListAuthorized(ctx context.Context) ([]sqlstore.AuthorizationRecord, error) {
	return []sqlstore.AuthorizationRecord{}, errors.New("error listing authorized")
}
