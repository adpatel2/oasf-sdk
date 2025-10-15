// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"github.com/agntcy/oasf-sdk/pkg/validator"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	MCPModuleName = "runtime/mcp"
	A2AModuleName = "runtime/a2a"
)

// RecordToGHCopilot translates a record into a GHCopilotMCPConfig structure.
func RecordToGHCopilot(record *structpb.Struct) (*GHCopilotMCPConfig, error) {
	// Get MCP module
	found, mcpModule := getModuleDataFromRecord(record, MCPModuleName)
	if !found {
		return nil, errors.New("MCP module not found in record")
	}

	// Process MCP module
	serversVal, ok := mcpModule.GetFields()["servers"]
	if !ok {
		return nil, errors.New("invalid or missing 'servers' in MCP module data")
	}

	serversStruct := serversVal.GetStructValue()
	if serversStruct == nil {
		return nil, errors.New("'servers' is not a struct")
	}

	servers := make(map[string]MCPServer)
	inputs := []MCPInput{}

	for serverName, serverVal := range serversStruct.Fields {
		serverMap := serverVal.GetStructValue()
		if serverMap == nil {
			continue
		}

		command, ok := serverMap.Fields["command"]
		if !ok {
			return nil, fmt.Errorf("missing 'command' for server '%s'", serverName)
		}

		args := []string{}
		if argsVal, ok := serverMap.Fields["args"]; ok {
			for _, arg := range argsVal.GetListValue().Values {
				args = append(args, arg.GetStringValue())
			}
		}

		env := map[string]string{}
		if envVal, ok := serverMap.Fields["env"]; ok {
			envStruct := envVal.GetStructValue()
			if envStruct != nil {
				for key, val := range envStruct.Fields {
					env[key] = val.GetStringValue()

					if after, ok0 := strings.CutPrefix(val.GetStringValue(), "${input:"); ok0 {
						id := strings.TrimSuffix(after, "}")
						inputs = append(inputs, MCPInput{
							ID:          id,
							Type:        "promptString",
							Password:    true,
							Description: fmt.Sprintf("Secret value for %s", id),
						})
					}
				}
			}
		}

		servers[serverName] = MCPServer{
			Command: command.GetStringValue(),
			Args:    args,
			Env:     env,
		}
	}

	return &GHCopilotMCPConfig{
		Servers: servers,
		Inputs:  inputs,
	}, nil
}

// RecordToA2A translates a record into an A2ACard structure.
func RecordToA2A(record *structpb.Struct) (*A2ACard, error) {
	// Get A2A module
	found, a2aModule := getModuleDataFromRecord(record, A2AModuleName)
	if !found {
		return nil, errors.New("A2A module not found in record")
	}

	// Process A2A module
	jsonBytes, err := json.Marshal(a2aModule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal A2A data to JSON: %w", err)
	}

	var card A2ACard
	if err := json.Unmarshal(jsonBytes, &card); err != nil {
		return nil, fmt.Errorf("failed to unmarshal A2A data into A2ACard: %w", err)
	}

	return &card, nil
}

// A2AToRecord translates an A2A card data back into an OASF-compliant record format.
func A2AToRecord(a2aData *structpb.Struct) (*structpb.Struct, error) {
	// Extract the a2aCard from the input data
	a2aCardVal, ok := a2aData.GetFields()["a2aCard"]
	if !ok {
		return nil, errors.New("missing 'a2aCard' in input data")
	}

	a2aCardStruct := a2aCardVal.GetStructValue()
	if a2aCardStruct == nil {
		return nil, errors.New("'a2aCard' is not a struct")
	}

	// Convert A2A card struct to map for easier access
	cardMap := a2aCardStruct.AsMap()

	// Extract name and description from A2A card for record metadata
	cardName := "generated-agent"
	cardDescription := "Agent generated from A2A card"

	if name, ok := cardMap["name"]; ok {
		if nameStr, ok := name.(string); ok {
			cardName = nameStr
		}
	}
	if description, ok := cardMap["description"]; ok {
		if descStr, ok := description.(string); ok {
			cardDescription = descStr
		}
	}

	// Create A2A data structure conforming to OASF v0.7.0 A2A data schema
	a2aModuleData := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"card_data": {
				Kind: &structpb.Value_StructValue{StructValue: a2aCardStruct},
			},
			"protocol_version": {
				Kind: &structpb.Value_StringValue{StringValue: "v1.0.0"},
			},
			"capabilities": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "streaming"}},
						},
					},
				},
			},
			"input_modes": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "text/plain"}},
							{Kind: &structpb.Value_StringValue{StringValue: "application/json"}},
						},
					},
				},
			},
			"output_modes": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "text/html"}},
							{Kind: &structpb.Value_StringValue{StringValue: "application/json"}},
						},
					},
				},
			},
			"security_schemes": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "none"}},
						},
					},
				},
			},
			"transports": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "http"}},
						},
					},
				},
			},
		},
	}

	// Create the A2A module with schema-compliant data
	a2aModule := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name": {
				Kind: &structpb.Value_StringValue{StringValue: A2AModuleName},
			},
			"version": {
				Kind: &structpb.Value_StringValue{StringValue: "v1.0.0"},
			},
			"data": {
				Kind: &structpb.Value_StructValue{StructValue: a2aModuleData},
			},
		},
	}

	// Create the modules list
	modulesList := &structpb.ListValue{
		Values: []*structpb.Value{
			{
				Kind: &structpb.Value_StructValue{StructValue: a2aModule},
			},
		},
	}

	// Create OASF-compliant record with all required fields
	record := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name": {
				Kind: &structpb.Value_StringValue{StringValue: cardName},
			},
			"schema_version": {
				Kind: &structpb.Value_StringValue{StringValue: "0.7.0"},
			},
			"version": {
				Kind: &structpb.Value_StringValue{StringValue: "v1.0.0"},
			},
			"description": {
				Kind: &structpb.Value_StringValue{StringValue: cardDescription},
			},
			"authors": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{
								Kind: &structpb.Value_StringValue{StringValue: "Generated by OASF SDK"},
							},
						},
					},
				},
			},
			"created_at": {
				Kind: &structpb.Value_StringValue{StringValue: "2025-10-06T00:00:00Z"},
			},
			"skills": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{Values: []*structpb.Value{}}, // Empty skills array
				},
			},
			"locators": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{Values: []*structpb.Value{}}, // Empty locators array
				},
			},
			"modules": {
				Kind: &structpb.Value_ListValue{ListValue: modulesList},
			},
		},
	}

	// Validate OASF compliance using the validator
	v, err := validator.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}
	if _, _, err := v.ValidateRecord(record); err != nil {
		return nil, fmt.Errorf("record validation failed: %w", err)
	}
	return record, nil
}

func getModuleDataFromRecord(record *structpb.Struct, moduleName string) (bool, *structpb.Struct) {
	// Find module by name
	modules, ok := record.GetFields()["modules"]
	if !ok {
		return false, nil
	}

	for _, module := range modules.GetListValue().Values {
		if strings.HasSuffix(module.GetStructValue().GetFields()["name"].GetStringValue(), moduleName) {
			return true, module.GetStructValue().GetFields()["data"].GetStructValue()
		}
	}
	return false, nil
}

// McpToRecord translates MCP data back into an OASF-compliant record format.
func McpToRecord(mcpData *structpb.Struct) (*structpb.Struct, error) {
    // Extract the mcpConfig from the input data
    mcpConfigVal, ok := mcpData.GetFields()["mcpConfig"]
    if !ok {
        return nil, errors.New("missing 'mcpConfig' in input data")
    }

    mcpConfigStruct := mcpConfigVal.GetStructValue()
    if mcpConfigStruct == nil {
        return nil, errors.New("'mcpConfig' is not a struct")
    }

    // Convert MCP config struct to map for easier access
    configMap := mcpConfigStruct.AsMap()

    // Extract name and description from MCP config for record metadata
    configName := "generated-mcp-agent"
    configDescription := "Agent generated from MCP configuration"

    if name, ok := configMap["name"]; ok {
        if nameStr, ok := name.(string); ok {
            configName = nameStr
        }
    }
    if description, ok := configMap["description"]; ok {
        if descStr, ok := description.(string); ok {
            configDescription = descStr
        }
    }

    // Create MCP servers array from the input data
    mcpServers, err := createMcpServersFromConfig(configMap)
    if err != nil {
        return nil, fmt.Errorf("failed to create MCP servers: %w", err)
    }

    // Create MCP data structure conforming to OASF v0.7.0 MCP data schema
    mcpModuleData := &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "servers": {
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{
                        Values: mcpServers,
                    },
                },
            },
        },
    }

    // Create the MCP module with schema-compliant data
    mcpModule := &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "name": {
                Kind: &structpb.Value_StringValue{StringValue: MCPModuleName},
            },
            "data": {
                Kind: &structpb.Value_StructValue{StructValue: mcpModuleData},
            },
        },
    }

    // Create the modules list
    modulesList := &structpb.ListValue{
        Values: []*structpb.Value{
            {
                Kind: &structpb.Value_StructValue{StructValue: mcpModule},
            },
        },
    }

    // Create OASF-compliant record with all required fields
    record := &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "name": {
                Kind: &structpb.Value_StringValue{StringValue: configName},
            },
            "schema_version": {
                Kind: &structpb.Value_StringValue{StringValue: "0.7.0"},
            },
            "version": {
                Kind: &structpb.Value_StringValue{StringValue: "v1.0.0"},
            },
            "description": {
                Kind: &structpb.Value_StringValue{StringValue: configDescription},
            },
            "authors": {
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{
                        Values: []*structpb.Value{
                            {
                                Kind: &structpb.Value_StringValue{StringValue: "Generated by OASF SDK"},
                            },
                        },
                    },
                },
            },
            "created_at": {
                Kind: &structpb.Value_StringValue{StringValue: "2025-10-06T00:00:00Z"},
            },
            "skills": {
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{Values: []*structpb.Value{}}, // Empty skills array
                },
            },
            "locators": {
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{Values: []*structpb.Value{}}, // Empty locators array
                },
            },
            "modules": {
                Kind: &structpb.Value_ListValue{ListValue: modulesList},
            },
        },
    }

    // Validate OASF compliance using the validator
    v, err := validator.New()
    if err != nil {
        return nil, fmt.Errorf("failed to create validator: %w", err)
    }
    if _, _, err := v.ValidateRecord(record); err != nil {
        return nil, fmt.Errorf("record validation failed: %w", err)
    }
    
    return record, nil
}

// createMcpServersFromConfig creates MCP servers array from configuration map
func createMcpServersFromConfig(configMap map[string]interface{}) ([]*structpb.Value, error) {
    var servers []*structpb.Value

    // Extract servers from config
    serversData, ok := configMap["servers"]
    if !ok {
        return nil, errors.New("missing 'servers' in MCP config")
    }

    // Handle servers as map[string]interface{} (server name -> server config)
    if serversMap, ok := serversData.(map[string]interface{}); ok {
        for serverName, serverConfig := range serversMap {
            server, err := createMcpServerFromConfig(serverName, serverConfig)
            if err != nil {
                return nil, fmt.Errorf("failed to create server '%s': %w", serverName, err)
            }
            servers = append(servers, server)
        }
    } else {
        return nil, errors.New("servers data is not a valid map")
    }

    return servers, nil
}

// createMcpServerFromConfig creates a single MCP server from configuration
func createMcpServerFromConfig(name string, serverConfig interface{}) (*structpb.Value, error) {
    configMap, ok := serverConfig.(map[string]interface{})
    if !ok {
        return nil, errors.New("server config is not a valid map")
    }

    serverFields := map[string]*structpb.Value{
        "name": {
            Kind: &structpb.Value_StringValue{StringValue: name},
        },
        "type": {
            Kind: &structpb.Value_StringValue{StringValue: getStringFromConfig(configMap, "type", "local")},
        },
        "capabilities": {
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{
                    Values: createCapabilitiesFromConfig(configMap),
                },
            },
        },
    }

    // Add optional fields if present
    if command, ok := configMap["command"].(string); ok {
        serverFields["command"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: command},
        }
    }

    if url, ok := configMap["url"].(string); ok {
        serverFields["url"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: url},
        }
    }

    if title, ok := configMap["title"].(string); ok {
        serverFields["title"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: title},
        }
    }

    if description, ok := configMap["description"].(string); ok {
        serverFields["description"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: description},
        }
    }

    if scope, ok := configMap["scope"].(string); ok {
        serverFields["scope"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: scope},
        }
    }

    // Add args if present
    if args := createArgsFromConfig(configMap); len(args) > 0 {
        serverFields["args"] = &structpb.Value{
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{Values: args},
            },
        }
    }

    // Add env_vars if present
    if envVars := createEnvVarsFromConfig(configMap); len(envVars) > 0 {
        serverFields["env_vars"] = &structpb.Value{
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{Values: envVars},
            },
        }
    }

    // Add tools if present
    if tools := createToolsFromConfig(configMap); len(tools) > 0 {
        serverFields["tools"] = &structpb.Value{
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{Values: tools},
            },
        }
    }

    // Add prompts if present
    if prompts := createPromptsFromConfig(configMap); len(prompts) > 0 {
        serverFields["prompts"] = &structpb.Value{
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{Values: prompts},
            },
        }
    }

    // Add resources if present
    if resources := createResourcesFromConfig(configMap); len(resources) > 0 {
        serverFields["resources"] = &structpb.Value{
            Kind: &structpb.Value_ListValue{
                ListValue: &structpb.ListValue{Values: resources},
            },
        }
    }

    // Add headers if present
    if headers := createHeadersFromConfig(configMap); headers != nil {
        serverFields["headers"] = headers
    }

    return &structpb.Value{
        Kind: &structpb.Value_StructValue{
            StructValue: &structpb.Struct{Fields: serverFields},
        },
    }, nil
}

// Helper functions for creating MCP server components

func getStringFromConfig(configMap map[string]interface{}, key, defaultValue string) string {
    if value, ok := configMap[key].(string); ok {
        return value
    }
    return defaultValue
}

func createCapabilitiesFromConfig(configMap map[string]interface{}) []*structpb.Value {
    capabilities := []*structpb.Value{
        {Kind: &structpb.Value_StringValue{StringValue: "notify"}},
        {Kind: &structpb.Value_StringValue{StringValue: "subscribe"}},
    }

    if capsData, ok := configMap["capabilities"]; ok {
        if capsList, ok := capsData.([]interface{}); ok {
            capabilities = nil
            for _, cap := range capsList {
                if capStr, ok := cap.(string); ok {
                    capabilities = append(capabilities, &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: capStr},
                    })
                }
            }
        }
    }

    return capabilities
}

func createArgsFromConfig(configMap map[string]interface{}) []*structpb.Value {
    var args []*structpb.Value
    if argsData, ok := configMap["args"]; ok {
        if argsList, ok := argsData.([]interface{}); ok {
            for _, arg := range argsList {
                if argStr, ok := arg.(string); ok {
                    args = append(args, &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: argStr},
                    })
                }
            }
        }
    }
    return args
}

func createEnvVarsFromConfig(configMap map[string]interface{}) []*structpb.Value {
    var envVars []*structpb.Value
    if envData, ok := configMap["env_vars"]; ok {
        if envList, ok := envData.([]interface{}); ok {
            for _, envVar := range envList {
                if envMap, ok := envVar.(map[string]interface{}); ok {
                    envVarStruct := createEnvVarStruct(envMap)
                    envVars = append(envVars, &structpb.Value{
                        Kind: &structpb.Value_StructValue{StructValue: envVarStruct},
                    })
                }
            }
        }
    }
    return envVars
}

func createEnvVarStruct(envMap map[string]interface{}) *structpb.Struct {
    fields := map[string]*structpb.Value{}

    if name, ok := envMap["name"].(string); ok {
        fields["name"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: name},
        }
    }

    if description, ok := envMap["description"].(string); ok {
        fields["description"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: description},
        }
    }

    if required, ok := envMap["required"].(bool); ok {
        fields["required"] = &structpb.Value{
            Kind: &structpb.Value_BoolValue{BoolValue: required},
        }
    }

    if defaultValue, ok := envMap["default_value"].(string); ok {
        fields["default_value"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: defaultValue},
        }
    }

    return &structpb.Struct{Fields: fields}
}

func createToolsFromConfig(configMap map[string]interface{}) []*structpb.Value {
    var tools []*structpb.Value
    if toolsData, ok := configMap["tools"]; ok {
        if toolsList, ok := toolsData.([]interface{}); ok {
            for _, tool := range toolsList {
                if toolMap, ok := tool.(map[string]interface{}); ok {
                    toolStruct := createToolStruct(toolMap)
                    tools = append(tools, &structpb.Value{
                        Kind: &structpb.Value_StructValue{StructValue: toolStruct},
                    })
                }
            }
        }
    }
    return tools
}

func createToolStruct(toolMap map[string]interface{}) *structpb.Struct {
    fields := map[string]*structpb.Value{}

    if name, ok := toolMap["name"].(string); ok {
        fields["name"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: name},
        }
    }

    if title, ok := toolMap["title"].(string); ok {
        fields["title"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: title},
        }
    }

    if description, ok := toolMap["description"].(string); ok {
        fields["description"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: description},
        }
    }

    if scopesData, ok := toolMap["scopes"]; ok {
        if scopesList, ok := scopesData.([]interface{}); ok {
            var scopes []*structpb.Value
            for _, scope := range scopesList {
                if scopeStr, ok := scope.(string); ok {
                    scopes = append(scopes, &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: scopeStr},
                    })
                }
            }
            fields["scopes"] = &structpb.Value{
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{Values: scopes},
                },
            }
        }
    }

    return &structpb.Struct{Fields: fields}
}

func createPromptsFromConfig(configMap map[string]interface{}) []*structpb.Value {
    var prompts []*structpb.Value
    if promptsData, ok := configMap["prompts"]; ok {
        if promptsList, ok := promptsData.([]interface{}); ok {
            for _, prompt := range promptsList {
                if promptMap, ok := prompt.(map[string]interface{}); ok {
                    promptStruct := createPromptStruct(promptMap)
                    prompts = append(prompts, &structpb.Value{
                        Kind: &structpb.Value_StructValue{StructValue: promptStruct},
                    })
                }
            }
        }
    }
    return prompts
}

func createPromptStruct(promptMap map[string]interface{}) *structpb.Struct {
    fields := map[string]*structpb.Value{}

    if name, ok := promptMap["name"].(string); ok {
        fields["name"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: name},
        }
    }

    if command, ok := promptMap["command"].(string); ok {
        fields["command"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: command},
        }
    }

    if description, ok := promptMap["description"].(string); ok {
        fields["description"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: description},
        }
    }

    if argsData, ok := promptMap["args"]; ok {
        if argsList, ok := argsData.([]interface{}); ok {
            var args []*structpb.Value
            for _, arg := range argsList {
                if argStr, ok := arg.(string); ok {
                    args = append(args, &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: argStr},
                    })
                }
            }
            fields["args"] = &structpb.Value{
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{Values: args},
                },
            }
        }
    }

    return &structpb.Struct{Fields: fields}
}

func createResourcesFromConfig(configMap map[string]interface{}) []*structpb.Value {
    var resources []*structpb.Value
    if resourcesData, ok := configMap["resources"]; ok {
        if resourcesList, ok := resourcesData.([]interface{}); ok {
            for _, resource := range resourcesList {
                if resourceMap, ok := resource.(map[string]interface{}); ok {
                    resourceStruct := createResourceStruct(resourceMap)
                    resources = append(resources, &structpb.Value{
                        Kind: &structpb.Value_StructValue{StructValue: resourceStruct},
                    })
                }
            }
        }
    }
    return resources
}

func createResourceStruct(resourceMap map[string]interface{}) *structpb.Struct {
    fields := map[string]*structpb.Value{}

    if name, ok := resourceMap["name"].(string); ok {
        fields["name"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: name},
        }
    }

    if title, ok := resourceMap["title"].(string); ok {
        fields["title"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: title},
        }
    }

    if description, ok := resourceMap["description"].(string); ok {
        fields["description"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: description},
        }
    }

    if uri, ok := resourceMap["uri"].(string); ok {
        fields["uri"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: uri},
        }
    }

    if uriTemplate, ok := resourceMap["uri_template"].(string); ok {
        fields["uri_template"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: uriTemplate},
        }
    }

    if mimeType, ok := resourceMap["mime_type"].(string); ok {
        fields["mime_type"] = &structpb.Value{
            Kind: &structpb.Value_StringValue{StringValue: mimeType},
        }
    }

    if priority, ok := resourceMap["priority"].(float64); ok {
        fields["priority"] = &structpb.Value{
            Kind: &structpb.Value_NumberValue{NumberValue: priority},
        }
    }

    if audienceData, ok := resourceMap["audience"]; ok {
        if audienceList, ok := audienceData.([]interface{}); ok {
            var audience []*structpb.Value
            for _, aud := range audienceList {
                if audStr, ok := aud.(string); ok {
                    audience = append(audience, &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: audStr},
                    })
                }
            }
            fields["audience"] = &structpb.Value{
                Kind: &structpb.Value_ListValue{
                    ListValue: &structpb.ListValue{Values: audience},
                },
            }
        }
    }

    return &structpb.Struct{Fields: fields}
}

func createHeadersFromConfig(configMap map[string]interface{}) *structpb.Value {
    if headersData, ok := configMap["headers"]; ok {
        if headersMap, ok := headersData.(map[string]interface{}); ok {
            fields := map[string]*structpb.Value{}
            for key, value := range headersMap {
                if valueStr, ok := value.(string); ok {
                    fields[key] = &structpb.Value{
                        Kind: &structpb.Value_StringValue{StringValue: valueStr},
                    }
                }
            }
            return &structpb.Value{
                Kind: &structpb.Value_StructValue{
                    StructValue: &structpb.Struct{Fields: fields},
                },
            }
        }
    }
    return nil
}