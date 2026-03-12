package metrics

import (
	"context"

	"github.com/go-tangra/go-tangra-executor/internal/data"
)

// Seed loads initial gauge values from the database.
// Called once at startup so Prometheus has accurate values from the start.
func (c *Collector) Seed(ctx context.Context, statsRepo *data.StatisticsRepo) {
	c.log.Info("Seeding Prometheus metrics from database...")

	scriptCount, err := statsRepo.GetGlobalScriptCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed script count: %v", err)
	} else {
		c.ScriptsTotal.Set(float64(scriptCount))
	}

	scriptsByEnabled, err := statsRepo.GetGlobalScriptCountByEnabled(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed scripts by enabled: %v", err)
	} else {
		for enabled, count := range scriptsByEnabled {
			c.ScriptsByEnabled.WithLabelValues(enabledLabel(enabled)).Set(float64(count))
		}
	}

	assignmentCount, err := statsRepo.GetGlobalAssignmentCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed assignment count: %v", err)
	} else {
		c.AssignmentsTotal.Set(float64(assignmentCount))
	}

	executionCount, err := statsRepo.GetGlobalExecutionCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed execution count: %v", err)
	} else {
		c.ExecutionsTotal.Set(float64(executionCount))
	}

	executionsByStatus, err := statsRepo.GetGlobalExecutionCountByStatus(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed executions by status: %v", err)
	} else {
		for status, count := range executionsByStatus {
			c.ExecutionsByStatus.WithLabelValues(status).Set(float64(count))
		}
	}

	c.log.Info("Prometheus metrics seeded successfully")
}
