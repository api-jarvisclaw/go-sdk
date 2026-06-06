package jarvisclaw

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse from chat/chat_completion.
type ChatResponse struct {
	Content string         `json:"-"`
	Model   string         `json:"model"`
	ID      string         `json:"id"`
	Usage   map[string]any `json:"usage"`
	Raw     map[string]any `json:"-"`
}

// ImageResponse from image_generate.
type ImageResponse struct {
	URL           string `json:"url"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// VideoJob from video_generate.
type VideoJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
}

// VideoRequest for video_generate.
type VideoRequest struct {
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration,omitempty"`
}

// SearchResult from search.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// AudioResponse from audio generation endpoints.
type AudioResponse struct {
	ID          string `json:"id,omitempty"`
	Status      string `json:"status,omitempty"`
	URL         string `json:"url,omitempty"`
	Data        []byte `json:"-"`
	ContentType string `json:"-"`
}

// Model from list_models.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}
