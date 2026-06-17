package tool

import (
	"context"
	"encoding/json"
)

type Definition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Tool interface {
	Definition() Definition
	Run(context.Context, json.RawMessage) (any, error)
}

func DefaultTools(bashConfig BashConfig) []Tool {
	return DefaultToolsWithBashOptions(bashConfig, BashOptions{})
}

func DefaultToolsWithBashOptions(bashConfig BashConfig, bashOptions BashOptions) []Tool {
	bashOptions.Config = bashConfig
	tools := []Tool{BashTool{Options: bashOptions}, ReadTool{}}
	if !bashConfig.ReadOnly {
		tools = append(tools, EditTool{})
	}
	return tools
}

func Handle(ctx context.Context, tools []Tool, name string, arguments json.RawMessage) string {
	for _, t := range tools {
		if t.Definition().Name != name {
			continue
		}
		result, err := t.Run(ctx, arguments)
		if err != nil {
			return ErrorJSON(err.Error())
		}
		return ResultJSON(result)
	}
	return ErrorJSON("unsupported tool call: " + name)
}

func ResultJSON(result any) string {
	b, err := json.Marshal(result)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	return string(b)
}

func ErrorJSON(msg string) string {
	b, _ := json.Marshal(map[string]any{"error": msg})
	return string(b)
}
