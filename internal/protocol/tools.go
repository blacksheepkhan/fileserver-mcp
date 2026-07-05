package protocol

type Tool struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}
