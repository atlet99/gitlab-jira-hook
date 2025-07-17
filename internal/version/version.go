// Package version provides version information and build details for the application.
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Constants
const (
	// ShortCommitHashLength defines the length for shortened commit hashes
	ShortCommitHashLength = 7
	// UnknownValue represents unknown build information
	UnknownValue = "unknown"
)

// Build-time variables set by linker flags
var (
	Version     = "dev"
	Commit      = "unknown"
	Date        = "unknown"
	BuiltBy     = "unknown"
	BuildNumber = "0"
)

// GetVersion returns the complete version string
func GetVersion() string {
	if BuildNumber != "0" && BuildNumber != "" {
		return fmt.Sprintf("%s (build %s)", Version, BuildNumber)
	}
	return Version
}

// GetFullVersionInfo returns detailed version information in modern format
func GetFullVersionInfo() string {
	var parts []string

	// Main version line
	versionLine := fmt.Sprintf("gitlab-jira-hook %s", GetVersion())
	if Commit != UnknownValue && Commit != "" {
		shortCommit := Commit
		if len(Commit) > ShortCommitHashLength {
			shortCommit = Commit[:ShortCommitHashLength]
		}
		versionLine += fmt.Sprintf(" (%s)", shortCommit)
	}
	parts = append(parts, versionLine)

	// Build info line
	var buildInfo []string
	if Date != UnknownValue && Date != "" {
		// Format date more nicely
		formattedDate := strings.ReplaceAll(Date, "_", " ")
		buildInfo = append(buildInfo, fmt.Sprintf("built %s", formattedDate))
	}
	if BuiltBy != UnknownValue && BuiltBy != "" {
		buildInfo = append(buildInfo, fmt.Sprintf("by %s", BuiltBy))
	}
	buildInfo = append(buildInfo,
		fmt.Sprintf("with %s", runtime.Version()),
		fmt.Sprintf("for %s/%s", runtime.GOOS, runtime.GOARCH))

	if len(buildInfo) > 0 {
		parts = append(parts, strings.Join(buildInfo, " "))
	}

	return strings.Join(parts, "\n")
}

// BuildInfo contains build information
type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	BuiltBy   string
	GoVersion string
	Platform  string
}

// Get returns the build information
func Get() *BuildInfo {
	return &BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string for the build info
func (bi *BuildInfo) String() string {
	return fmt.Sprintf("gitlab-jira-hook version %s (%s) built on %s by %s using %s for %s",
		bi.Version, bi.Commit, bi.Date, bi.BuiltBy, bi.GoVersion, bi.Platform)
}

// Short returns a short version string
func (bi *BuildInfo) Short() string {
	return fmt.Sprintf("gitlab-jira-hook %s", bi.Version)
}
