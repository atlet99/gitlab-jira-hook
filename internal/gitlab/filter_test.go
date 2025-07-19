package gitlab

import (
	"testing"

	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestIsAllowedEvent_Handler(t *testing.T) {
	tests := []struct {
		name            string
		allowedProjects []string
		allowedGroups   []string
		event           *Event
		want            bool
	}{
		{
			name:            "no filters, allow all",
			allowedProjects: nil,
			allowedGroups:   nil,
			event:           &Event{Project: &Project{Name: "foo", PathWithNamespace: "group/foo"}},
			want:            true,
		},
		{
			name:            "project allowed by name",
			allowedProjects: []string{"foo"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{Name: "foo"}},
			want:            true,
		},
		{
			name:            "project allowed by path",
			allowedProjects: []string{"group/foo"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{PathWithNamespace: "group/foo"}},
			want:            true,
		},
		{
			name:            "project allowed by group prefix",
			allowedProjects: []string{"devops"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{PathWithNamespace: "devops/login/stg"}},
			want:            true,
		},
		{
			name:            "group allowed by name",
			allowedProjects: nil,
			allowedGroups:   []string{"bar-group"},
			event:           &Event{Group: &Group{Name: "bar-group"}},
			want:            true,
		},
		{
			name:            "group allowed by full path",
			allowedProjects: nil,
			allowedGroups:   []string{"org/bar-group"},
			event:           &Event{Group: &Group{FullPath: "org/bar-group"}},
			want:            true,
		},
		{
			name:            "not allowed",
			allowedProjects: []string{"foo"},
			allowedGroups:   []string{"bar-group"},
			event:           &Event{Project: &Project{Name: "baz"}},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				config: &config.Config{
					AllowedProjects: tt.allowedProjects,
					AllowedGroups:   tt.allowedGroups,
				},
				logger: slog.Default(),
			}
			got := h.isAllowedEvent(tt.event)
			if got != tt.want {
				t.Errorf("isAllowedEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllowedEvent_ProjectHookHandler(t *testing.T) {
	tests := []struct {
		name            string
		allowedProjects []string
		allowedGroups   []string
		event           *Event
		want            bool
	}{
		{
			name:            "no filters, allow all",
			allowedProjects: nil,
			allowedGroups:   nil,
			event:           &Event{Project: &Project{Name: "foo", PathWithNamespace: "group/foo"}},
			want:            true,
		},
		{
			name:            "project allowed by name",
			allowedProjects: []string{"foo"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{Name: "foo"}},
			want:            true,
		},
		{
			name:            "project allowed by path",
			allowedProjects: []string{"group/foo"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{PathWithNamespace: "group/foo"}},
			want:            true,
		},
		{
			name:            "project allowed by group prefix",
			allowedProjects: []string{"devops"},
			allowedGroups:   nil,
			event:           &Event{Project: &Project{PathWithNamespace: "devops/login/stg"}},
			want:            true,
		},
		{
			name:            "group allowed by name",
			allowedProjects: nil,
			allowedGroups:   []string{"bar-group"},
			event:           &Event{Group: &Group{Name: "bar-group"}},
			want:            true,
		},
		{
			name:            "group allowed by full path",
			allowedProjects: nil,
			allowedGroups:   []string{"org/bar-group"},
			event:           &Event{Group: &Group{FullPath: "org/bar-group"}},
			want:            true,
		},
		{
			name:            "not allowed",
			allowedProjects: []string{"foo"},
			allowedGroups:   []string{"bar-group"},
			event:           &Event{Project: &Project{Name: "baz"}},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &ProjectHookHandler{
				config: &config.Config{
					AllowedProjects: tt.allowedProjects,
					AllowedGroups:   tt.allowedGroups,
				},
				logger: slog.Default(),
			}
			got := h.isAllowedEvent(tt.event)
			if got != tt.want {
				t.Errorf("isAllowedEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
