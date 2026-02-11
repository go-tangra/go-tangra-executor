//go:build wireinject
// +build wireinject

//go:generate go run github.com/google/wire/cmd/wire

package providers

import (
	"github.com/google/wire"

	"github.com/go-tangra/go-tangra-executor/internal/service"
)

// ProviderSet is the Wire provider set for service layer
var ProviderSet = wire.NewSet(
	service.NewCommandRegistry,
	service.NewScriptService,
	service.NewAssignmentService,
	service.NewExecutionService,
	service.NewClientService,
)
