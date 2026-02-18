package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-common/grpcx"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/executionlog"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/script"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/scriptassignment"
)

const (
	backupModule  = "executor"
	backupVersion = "1.0"
)

type BackupService struct {
	executorV1.UnimplementedBackupServiceServer

	log       *log.Helper
	entClient *entCrud.EntClient[*ent.Client]
}

func NewBackupService(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BackupService {
	return &BackupService{
		log:       ctx.NewLoggerHelper("executor/service/backup"),
		entClient: entClient,
	}
}

type backupData struct {
	Module     string         `json:"module"`
	Version    string         `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	TenantID   uint32         `json:"tenantId"`
	FullBackup bool           `json:"fullBackup"`
	Data       backupEntities `json:"data"`
}

type backupEntities struct {
	Scripts           []json.RawMessage `json:"scripts,omitempty"`
	ScriptAssignments []json.RawMessage `json:"scriptAssignments,omitempty"`
	ExecutionLogs     []json.RawMessage `json:"executionLogs,omitempty"`
}

func marshalEntities[T any](entities []*T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(entities))
	for _, e := range entities {
		b, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

func (s *BackupService) ExportBackup(ctx context.Context, req *executorV1.ExportBackupRequest) (*executorV1.ExportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	full := false

	if grpcx.IsPlatformAdmin(ctx) && req.TenantId != nil && *req.TenantId == 0 {
		full = true
		tenantID = 0
	} else if req.TenantId != nil && *req.TenantId != 0 {
		if grpcx.IsPlatformAdmin(ctx) {
			tenantID = *req.TenantId
		}
	}

	client := s.entClient.Client()
	now := time.Now()

	scripts, err := s.exportScripts(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export scripts: %w", err)
	}
	scriptAssignments, err := s.exportScriptAssignments(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export script assignments: %w", err)
	}
	executionLogs, err := s.exportExecutionLogs(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export execution logs: %w", err)
	}

	backup := backupData{
		Module:     backupModule,
		Version:    backupVersion,
		ExportedAt: now,
		TenantID:   tenantID,
		FullBackup: full,
		Data: backupEntities{
			Scripts:           scripts,
			ScriptAssignments: scriptAssignments,
			ExecutionLogs:     executionLogs,
		},
	}

	data, err := json.Marshal(backup)
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}

	entityCounts := map[string]int64{
		"scripts":           int64(len(scripts)),
		"scriptAssignments": int64(len(scriptAssignments)),
		"executionLogs":     int64(len(executionLogs)),
	}

	s.log.Infof("exported backup: module=%s tenant=%d full=%v entities=%v", backupModule, tenantID, full, entityCounts)

	return &executorV1.ExportBackupResponse{
		Data:         data,
		Module:       backupModule,
		Version:      backupVersion,
		ExportedAt:   timestamppb.New(now),
		TenantId:     tenantID,
		EntityCounts: entityCounts,
	}, nil
}

func (s *BackupService) ImportBackup(ctx context.Context, req *executorV1.ImportBackupRequest) (*executorV1.ImportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	isPlatformAdmin := grpcx.IsPlatformAdmin(ctx)
	mode := req.GetMode()

	var backup backupData
	if err := json.Unmarshal(req.GetData(), &backup); err != nil {
		return nil, fmt.Errorf("invalid backup data: %w", err)
	}

	if backup.Module != backupModule {
		return nil, fmt.Errorf("backup module mismatch: expected %s, got %s", backupModule, backup.Module)
	}
	if backup.Version != backupVersion {
		return nil, fmt.Errorf("backup version mismatch: expected %s, got %s", backupVersion, backup.Version)
	}

	if backup.FullBackup && !isPlatformAdmin {
		return nil, fmt.Errorf("only platform admins can restore full backups")
	}

	if !isPlatformAdmin || !backup.FullBackup {
		tenantID = grpcx.GetTenantIDFromContext(ctx)
	} else {
		tenantID = 0
	}

	client := s.entClient.Client()
	var results []*executorV1.EntityImportResult
	var warnings []string

	importFuncs := []struct {
		name  string
		items []json.RawMessage
		fn    func(context.Context, *ent.Client, []json.RawMessage, uint32, bool, executorV1.RestoreMode) (*executorV1.EntityImportResult, []string)
	}{
		{"scripts", backup.Data.Scripts, s.importScripts},
		{"scriptAssignments", backup.Data.ScriptAssignments, s.importScriptAssignments},
		{"executionLogs", backup.Data.ExecutionLogs, s.importExecutionLogs},
	}

	for _, imp := range importFuncs {
		if len(imp.items) == 0 {
			continue
		}
		result, w := imp.fn(ctx, client, imp.items, tenantID, backup.FullBackup, mode)
		if result != nil {
			results = append(results, result)
		}
		warnings = append(warnings, w...)
	}

	s.log.Infof("imported backup: module=%s tenant=%d mode=%v results=%d warnings=%d", backupModule, tenantID, mode, len(results), len(warnings))

	return &executorV1.ImportBackupResponse{
		Success:  true,
		Results:  results,
		Warnings: warnings,
	}, nil
}

// --- Export helpers ---

func (s *BackupService) exportScripts(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Script.Query()
	if !full {
		query = query.Where(script.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportScriptAssignments(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.ScriptAssignment.Query()
	if !full {
		query = query.Where(scriptassignment.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportExecutionLogs(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.ExecutionLog.Query()
	if !full {
		query = query.Where(executionlog.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

// --- Import helpers ---

func (s *BackupService) importScripts(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode executorV1.RestoreMode) (*executorV1.EntityImportResult, []string) {
	result := &executorV1.EntityImportResult{EntityType: "scripts", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.Script
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("scripts: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Script.Get(ctx, e.ID)
		if existing != nil {
			if mode == executorV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Script.UpdateOneID(e.ID).
				SetName(e.Name).
				SetDescription(e.Description).
				SetScriptType(e.ScriptType).
				SetContent(e.Content).
				SetContentHash(e.ContentHash).
				SetVersion(e.Version).
				SetEnabled(e.Enabled).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("scripts: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Script.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetDescription(e.Description).
				SetScriptType(e.ScriptType).
				SetContent(e.Content).
				SetContentHash(e.ContentHash).
				SetVersion(e.Version).
				SetEnabled(e.Enabled).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("scripts: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importScriptAssignments(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode executorV1.RestoreMode) (*executorV1.EntityImportResult, []string) {
	result := &executorV1.EntityImportResult{EntityType: "scriptAssignments", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.ScriptAssignment
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("scriptAssignments: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.ScriptAssignment.Get(ctx, e.ID)
		if existing != nil {
			if mode == executorV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.ScriptAssignment.UpdateOneID(e.ID).
				SetScriptID(e.ScriptID).
				SetClientID(e.ClientID).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("scriptAssignments: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.ScriptAssignment.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetScriptID(e.ScriptID).
				SetClientID(e.ClientID).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("scriptAssignments: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importExecutionLogs(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode executorV1.RestoreMode) (*executorV1.EntityImportResult, []string) {
	result := &executorV1.EntityImportResult{EntityType: "executionLogs", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.ExecutionLog
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("executionLogs: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.ExecutionLog.Get(ctx, e.ID)
		if existing != nil {
			if mode == executorV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.ExecutionLog.UpdateOneID(e.ID).
				SetScriptID(e.ScriptID).
				SetScriptName(e.ScriptName).
				SetClientID(e.ClientID).
				SetScriptHash(e.ScriptHash).
				SetTriggerType(e.TriggerType).
				SetStatus(e.Status).
				SetNillableExitCode(e.ExitCode).
				SetOutput(e.Output).
				SetErrorOutput(e.ErrorOutput).
				SetRejectionReason(e.RejectionReason).
				SetNillableStartedAt(e.StartedAt).
				SetNillableCompletedAt(e.CompletedAt).
				SetNillableDurationMs(e.DurationMs).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("executionLogs: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.ExecutionLog.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetScriptID(e.ScriptID).
				SetScriptName(e.ScriptName).
				SetClientID(e.ClientID).
				SetScriptHash(e.ScriptHash).
				SetTriggerType(e.TriggerType).
				SetStatus(e.Status).
				SetNillableExitCode(e.ExitCode).
				SetOutput(e.Output).
				SetErrorOutput(e.ErrorOutput).
				SetRejectionReason(e.RejectionReason).
				SetNillableStartedAt(e.StartedAt).
				SetNillableCompletedAt(e.CompletedAt).
				SetNillableDurationMs(e.DurationMs).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("executionLogs: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}
