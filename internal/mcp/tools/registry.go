package tools

import "sync"

// Registry stores MCP tools.
type Registry struct {
	tools map[string]Tool
	order []string
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
		order: make([]string, 0),
	}
}

// Register registers or replaces a tool.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()

	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}

	r.tools[name] = tool
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools in registration order.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.order))

	for _, name := range r.order {
		tool, ok := r.tools[name]
		if ok {
			result = append(result, tool)
		}
	}

	return result
}
