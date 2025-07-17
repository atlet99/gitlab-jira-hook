package jira

// CommentPayload represents the payload for adding a comment
type CommentPayload struct {
	Body CommentBody `json:"body"`
}

// CommentBody represents the body of a comment
type CommentBody struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []Content `json:"content"`
}

// Content represents content in a comment
type Content struct {
	Type    string        `json:"type"`
	Content []TextContent `json:"content"`
}

// TextContent represents text content
// Marks for formatting (bold, link, etc.)
type TextContent struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Marks []Mark `json:"marks,omitempty"`
}

// Mark describes text formatting (bold, link, etc.)
type Mark struct {
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs,omitempty"`
}
