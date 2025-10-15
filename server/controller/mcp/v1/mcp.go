// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agntcy/oasf-sdk/pkg/translator"
	mcpv1 "github.com/agntcy/oasf-sdk/server/gen/agntcy/oasfsdk/mcp/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type mcpCtrl struct {
	mcpv1.UnimplementedMCPServiceServer
}

func NewMCPController() mcpv1.MCPServiceServer {
	return &mcpCtrl{}
}

func (c *mcpCtrl) MCPToRecord(ctx context.Context, req *mcpv1.MCPToRecordRequest) (*mcpv1.MCPToRecordResponse, error) {
	slog.Info("MCPToRecord request received")

	if req.McpConfig == nil {
		return nil, fmt.Errorf("mcp_config is required")
	}

	// The translator expects the data to have an "mcpConfig" key,
	// so we need to wrap the input properly
	wrappedData, err := structpb.NewStruct(map[string]interface{}{
		"mcpConfig": req.McpConfig.AsMap(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create wrapped structure: %w", err)
	}

	// Call the translator with the wrapped structure
	record, err := translator.McpToRecord(wrappedData)
	if err != nil {
		slog.Error("Failed to translate MCP to record", "error", err)
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	slog.Info("MCPToRecord translation completed successfully")
	return &mcpv1.MCPToRecordResponse{
		Record: record,
	}, nil
}
