package gitlab

// Event represents a GitLab webhook event
type Event struct {
	ObjectKind       string            `json:"object_kind"`
	EventName        string            `json:"event_name"`
	Type             string            `json:"-"` // Computed field
	Commits          []Commit          `json:"commits"`
	ObjectAttributes *ObjectAttributes `json:"object_attributes"`
}

// Commit represents a Git commit
type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Author  Author `json:"author"`
}

// Author represents a commit author
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ObjectAttributes represents merge request attributes
type ObjectAttributes struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Action string `json:"action"`
	URL    string `json:"url"`
}
