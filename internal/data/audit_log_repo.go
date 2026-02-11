package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-executor/internal/data/ent"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/auditlog"

	"github.com/go-tangra/go-tangra-common/middleware/audit"
	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// AuditLogRepo implements audit.AuditLogRepository for executor
type AuditLogRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewAuditLogRepo creates a new AuditLogRepo
func NewAuditLogRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AuditLogRepo {
	return &AuditLogRepo{
		log:       ctx.NewLoggerHelper("executor/audit_log_repo"),
		entClient: entClient,
	}
}

// CreateFromEntry implements audit.AuditLogRepository
func (r *AuditLogRepo) CreateFromEntry(ctx context.Context, entry *audit.AuditLogEntry) error {
	builder := r.entClient.Client().AuditLog.Create().
		SetAuditID(entry.AuditID).
		SetOperation(entry.Operation).
		SetServiceName(entry.ServiceName).
		SetSuccess(entry.Success).
		SetIsAuthenticated(entry.IsAuthenticated).
		SetLatencyMs(entry.LatencyMs).
		SetCreateTime(entry.Timestamp)

	if entry.TenantID > 0 {
		builder.SetTenantID(entry.TenantID)
	}
	if entry.RequestID != "" {
		builder.SetRequestID(entry.RequestID)
	}
	if entry.ClientID != "" {
		builder.SetClientID(entry.ClientID)
	}
	if entry.ClientCommonName != "" {
		builder.SetClientCommonName(entry.ClientCommonName)
	}
	if entry.ClientOrganization != "" {
		builder.SetClientOrganization(entry.ClientOrganization)
	}
	if entry.ClientSerialNumber != "" {
		builder.SetClientSerialNumber(entry.ClientSerialNumber)
	}
	if entry.ErrorCode != 0 {
		builder.SetErrorCode(entry.ErrorCode)
	}
	if entry.ErrorMessage != "" {
		builder.SetErrorMessage(entry.ErrorMessage)
	}
	if entry.PeerAddress != "" {
		builder.SetPeerAddress(entry.PeerAddress)
	}
	if entry.GeoLocation != nil {
		builder.SetGeoLocation(entry.GeoLocation)
	}
	if entry.LogHash != "" {
		builder.SetLogHash(entry.LogHash)
	}
	if entry.Signature != nil {
		builder.SetSignature(entry.Signature)
	}
	if entry.Metadata != nil {
		builder.SetMetadata(entry.Metadata)
	}

	_, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create audit log failed: %s", err.Error())
		return err
	}

	return nil
}

// List retrieves audit logs with filtering options
func (r *AuditLogRepo) List(ctx context.Context, tenantID *uint32, clientID, operation *string, success *bool, limit, offset int) ([]*ent.AuditLog, int, error) {
	query := r.entClient.Client().AuditLog.Query()

	if tenantID != nil {
		query = query.Where(auditlog.TenantIDEQ(*tenantID))
	}
	if clientID != nil {
		query = query.Where(auditlog.ClientIDEQ(*clientID))
	}
	if operation != nil {
		query = query.Where(auditlog.OperationContains(*operation))
	}
	if success != nil {
		query = query.Where(auditlog.SuccessEQ(*success))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count audit logs failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("count audit logs failed")
	}

	query = query.Order(ent.Desc(auditlog.FieldCreateTime))
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	entities, err := query.All(ctx)
	if err != nil {
		r.log.Errorf("list audit logs failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("list audit logs failed")
	}

	return entities, total, nil
}

// DeleteOlderThan deletes audit logs older than the specified time
func (r *AuditLogRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int, error) {
	deleted, err := r.entClient.Client().AuditLog.Delete().
		Where(auditlog.CreateTimeLT(before)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete old audit logs failed: %s", err.Error())
		return 0, executorV1.ErrorInternalServerError("delete old audit logs failed")
	}
	return deleted, nil
}
