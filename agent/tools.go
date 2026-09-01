package agent

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry(
	tools ...Tool,
) *ToolRegistry {

	registry := &ToolRegistry{
		tools: make(
			map[string]Tool,
			len(tools),
		),
	}

	for _, tool := range tools {
		registry.tools[tool.ID()] = tool
	}

	return registry
}

func (r *ToolRegistry) Get(
	name string,
) (Tool, bool) {

	tool, ok :=
		r.tools[name]

	return tool, ok
}

func (r *ToolRegistry) Definitions() []ToolDefinition {

	result :=
		make(
			[]ToolDefinition,
			0,
			len(r.tools),
		)

	for _, tool := range r.tools {

		result =
			append(
				result,
				ToolDefinition{
					Name: tool.ID(),

					Description: tool.Description(),

					Parameters: tool.Parameters(),
				},
			)
	}

	return result
}
