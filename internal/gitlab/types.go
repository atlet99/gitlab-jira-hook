package gitlab

// Event represents a GitLab webhook event
type Event struct {
	ObjectKind       string            `json:"object_kind"`
	EventName        string            `json:"event_name"`
	Type             string            `json:"-"` // Computed field
	Commits          []Commit          `json:"commits"`
	ObjectAttributes *ObjectAttributes `json:"object_attributes"`
	// Project events
	Project *Project `json:"project"`
	// User events
	User *User `json:"user"`
	// Team/Group events
	Team  *Team  `json:"team"`
	Group *Group `json:"group"`
	// Issue events
	Issue *Issue `json:"issue"`
	// Merge Request events
	MergeRequest *MergeRequest `json:"merge_request"`
	// Note/Comment events
	Note *Note `json:"note"`
	// Wiki Page events
	WikiPage *WikiPage `json:"wiki_page"`
	// Pipeline events
	Pipeline *Pipeline `json:"pipeline"`
	// Build/Job events
	Build *Build `json:"build"`
	// Release events
	Release *Release `json:"release"`
	// Deployment events
	Deployment *Deployment `json:"deployment"`
	// Feature Flag events
	FeatureFlag *FeatureFlag `json:"feature_flag"`
	// Repository events
	Repository *Repository `json:"repository"`
	//Environment events
	Environment *Environment `json:"environment"`
	// Additional metadata
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// System Hook specific fields
	EventType string `json:"event_type"`
	// User events specific
	Email string `json:"email"`
	// Project events specific
	PathWithNamespace string `json:"path_with_namespace"`
	// Group events specific
	FullPath string `json:"full_path"`
	FullName string `json:"full_name"`
	// Team events specific
	TeamPath string `json:"team_path"`
	// Repository events specific
	GitSSHURL     string `json:"git_ssh_url"`
	GitHTTPURL    string `json:"git_http_url"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
	// User membership events
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	// Project membership events
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
	// Group membership events
	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	// Team membership events
	TeamID   int    `json:"team_id"`
	TeamName string `json:"team_name"`
	// Access level for membership events
	AccessLevel int `json:"access_level"`
	// Project access level
	ProjectAccessLevel int `json:"project_access_level"`
	// Group access level
	GroupAccessLevel int `json:"group_access_level"`
	// System Hook specific fields from documentation
	// Project events
	Name                 string `json:"name"`
	Path                 string `json:"path"`
	ProjectVisibility    string `json:"project_visibility"`
	OwnerEmail           string `json:"owner_email"`
	OwnerName            string `json:"owner_name"`
	Owners               []User `json:"owners"`
	OldPathWithNamespace string `json:"old_path_with_namespace"`
	// User events
	OldUsername string `json:"old_username"`
	State       string `json:"state"`
	// Key events
	Key   string `json:"key"`
	KeyID int    `json:"id"`
	// Group events
	OldPath     string `json:"old_path"`
	OldFullPath string `json:"old_full_path"`
	// Push events
	Before            string `json:"before"`
	After             string `json:"after"`
	Ref               string `json:"ref"`
	CheckoutSha       string `json:"checkout_sha"`
	UserAvatar        string `json:"user_avatar"`
	TotalCommitsCount int    `json:"total_commits_count"`
	// Tag events
	TagPush bool `json:"tag_push"`
}

// Commit represents a Git commit
type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Author  Author `json:"author"`
	// Additional commit fields
	Timestamp string   `json:"timestamp"`
	Added     []string `json:"added"`
	Modified  []string `json:"modified"`
	Removed   []string `json:"removed"`
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
	// Additional fields for project hooks
	Note        string `json:"note"`
	Content     string `json:"content"`
	Ref         string `json:"ref"`
	Status      string `json:"status"`
	Name        string `json:"name"`
	Stage       string `json:"stage"`
	Environment string `json:"environment"`
	Description string `json:"description"`
	// Merge Request specific fields
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	MergeStatus  string `json:"merge_status"`
	// Issue specific fields
	IssueType string `json:"issue_type"`
	Priority  string `json:"priority"`
	// Pipeline specific fields
	Sha       string     `json:"sha"`
	Duration  int        `json:"duration"`
	Variables []Variable `json:"variables"`
}

// Variable represents a pipeline variable
type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Issue represents a GitLab issue
type Issue struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	URL         string `json:"url"`
	// Additional issue fields
	IssueType string   `json:"issue_type"`
	Priority  string   `json:"priority"`
	Labels    []string `json:"labels"`
	Assignee  *User    `json:"assignee"`
	Author    *User    `json:"author"`
}

// MergeRequest represents a GitLab merge request
type MergeRequest struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	URL         string `json:"url"`
	// Additional merge request fields
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	MergeStatus  string   `json:"merge_status"`
	Assignee     *User    `json:"assignee"`
	Author       *User    `json:"author"`
	Labels       []string `json:"labels"`
}

// Note represents a GitLab comment/note
type Note struct {
	ID       int    `json:"id"`
	Note     string `json:"note"`
	Noteable string `json:"noteable_type"`
	URL      string `json:"url"`
	// Additional note fields
	Author *User `json:"author"`
}

// WikiPage represents a GitLab wiki page
type WikiPage struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url"`
	// Additional wiki page fields
	Action string `json:"action"`
	Author *User  `json:"author"`
}

// Pipeline represents a GitLab pipeline
type Pipeline struct {
	ID       int    `json:"id"`
	Ref      string `json:"ref"`
	Status   string `json:"status"`
	URL      string `json:"url"`
	Duration int    `json:"duration"`
	// Additional pipeline fields
	Sha       string     `json:"sha"`
	Variables []Variable `json:"variables"`
	Author    *User      `json:"author"`
}

// Build represents a GitLab build/job
type Build struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Stage    string `json:"stage"`
	Status   string `json:"status"`
	URL      string `json:"url"`
	Duration int    `json:"duration"`
	// Additional build fields
	Ref    string  `json:"ref"`
	Sha    string  `json:"sha"`
	Author *User   `json:"author"`
	Runner *Runner `json:"runner"`
}

// Runner represents a GitLab runner
type Runner struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	IsShared    bool   `json:"is_shared"`
}

// Release represents a GitLab release
type Release struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	// Additional release fields
	Author *User `json:"author"`
}

// Deployment represents a GitLab deployment
type Deployment struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	// Additional deployment fields
	Sha    string `json:"sha"`
	Author *User  `json:"author"`
}

// FeatureFlag represents a GitLab feature flag
type FeatureFlag struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	// Additional feature flag fields
	Author *User `json:"author"`
}

// Repository represents a GitLab repository
type Repository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	// Additional repository fields
	GitSSHURL     string `json:"git_ssh_url"`
	GitHTTPURL    string `json:"git_http_url"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
}

// Environment represents a GitLab environment
type Environment struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	State string `json:"state"`
	// Additional environment fields
	ExternalURL string `json:"external_url"`
}

// Project represents a GitLab project
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	AvatarURL         string `json:"avatar_url"`
	GitSSHURL         string `json:"git_ssh_url"`
	GitHTTPURL        string `json:"git_http_url"`
	Namespace         string `json:"namespace"`
	VisibilityLevel   int    `json:"visibility_level"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	Homepage          string `json:"homepage"`
	URL               string `json:"url"`
	SSHURL            string `json:"ssh_url"`
	HTTPURL           string `json:"http_url"`
	// Additional project fields
	Visibility string `json:"visibility"`
	Owner      *User  `json:"owner"`
}

// User represents a GitLab user
type User struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	// Additional user fields
	State string `json:"state"`
}

// Team represents a GitLab team
type Team struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	WebURL      string `json:"web_url"`
	// Additional team fields
	AvatarURL string `json:"avatar_url"`
}

// Group represents a GitLab group
type Group struct {
	ID                   int    `json:"id"`
	Name                 string `json:"name"`
	Path                 string `json:"path"`
	Description          string `json:"description"`
	Visibility           string `json:"visibility"`
	WebURL               string `json:"web_url"`
	AvatarURL            string `json:"avatar_url"`
	FullPath             string `json:"full_path"`
	FullName             string `json:"full_name"`
	ParentID             int    `json:"parent_id"`
	LFSEnabled           bool   `json:"lfs_enabled"`
	RequestAccessEnabled bool   `json:"request_access_enabled"`
	// Additional group fields
	Parent *Group `json:"parent"`
}
