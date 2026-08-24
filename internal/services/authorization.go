package services

import (
	"strings"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
)

var defaultRolePermissions = map[string][]string{
	"org_admin": {
		"projects:view", "projects:add", "projects:modify", "projects:delete",
		"sprints:view", "sprints:add", "sprints:modify", "sprints:delete",
		"user_stories:view", "user_stories:add", "user_stories:modify", "user_stories:delete",
		"tasks:view", "tasks:add", "tasks:modify", "tasks:delete",
		"comments:view", "comments:add", "comments:modify", "comments:delete",
		"attachments:view", "attachments:add", "attachments:delete",
		"custom_statuses:view", "custom_statuses:modify",
	},
	"project_manager": {
		"projects:view", "projects:modify",
		"sprints:view", "sprints:add", "sprints:modify", "sprints:delete",
		"user_stories:view", "user_stories:add", "user_stories:modify", "user_stories:delete",
		"tasks:view", "tasks:add", "tasks:modify", "tasks:delete",
		"comments:view", "comments:add", "comments:modify", "comments:delete",
		"attachments:view", "attachments:add", "attachments:delete",
		"custom_statuses:view", "custom_statuses:modify",
	},
	"developer": {
		"projects:view",
		"sprints:view",
		"user_stories:view", "user_stories:add", "user_stories:modify",
		"tasks:view", "tasks:add", "tasks:modify", "tasks:delete",
		"comments:view", "comments:add", "comments:modify", "comments:delete",
		"attachments:view", "attachments:add", "attachments:delete",
		"custom_statuses:view",
	},
	"qa": {
		"projects:view",
		"sprints:view",
		"user_stories:view", "user_stories:modify",
		"tasks:view", "tasks:add", "tasks:modify",
		"comments:view", "comments:add",
		"attachments:view", "attachments:add",
		"custom_statuses:view",
	},
	"stakeholder": {
		"projects:view",
		"sprints:view",
		"user_stories:view",
		"tasks:view",
		"comments:view", "comments:add",
		"attachments:view",
		"custom_statuses:view",
	},
}

func hasDefaultPermission(roleName, resource, action string) bool {
	name := strings.ToLower(roleName)
	if name == "member" || name == "user" {
		name = "developer"
	}
	if name == "tester" {
		name = "qa"
	}
	if name == "viewer" {
		name = "stakeholder"
	}

	perms, ok := defaultRolePermissions[name]
	if !ok {
		return false
	}
	target := resource + ":" + action
	for _, p := range perms {
		if p == target {
			return true
		}
	}
	return false
}

func CheckPermission(authRepo authrepo.AuthRepository, projectRepo projectrepo.ProjectRepository, userID, projectID uuid.UUID, resource string, action string) (bool, *response.Error) {
	user, err := authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	// 1. Super admins are platform-level only, they cannot perform org/project activities
	if user.Role.Name == "super_admin" {
		return false, nil
	}

	// 2. If projectID is not Nil, check project-level role first
	if projectID != uuid.Nil && projectRepo != nil {
		member, err := projectRepo.GetProjectMemberByUserAndProjectID(userID, projectID)
		if err == nil && member != nil {
			for _, p := range member.Role.Permissions {
				if p.Resource == resource && p.Action == action {
					return true, nil
				}
			}
			if hasDefaultPermission(member.Role.Name, resource, action) {
				return true, nil
			}
		}
	}

	// 3. Otherwise, check organization-level role if organization matches
	if projectID != uuid.Nil && projectRepo != nil {
		project, err := projectRepo.GetProjectByID(projectID)
		if err == nil {
			if user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
				// Only Org Admins fallback to organization role permissions for project context
				if user.Role.Name == "org_admin" {
					for _, p := range user.Role.Permissions {
						if p.Resource == resource && p.Action == action {
							return true, nil
						}
					}
					if hasDefaultPermission(user.Role.Name, resource, action) {
						return true, nil
					}
				}
			}
		} else {
			return false, err
		}
	} else {
		// If no project ID (e.g. creating/listing projects), we check the user's organization-level role directly.
		for _, p := range user.Role.Permissions {
			if p.Resource == resource && p.Action == action {
				return true, nil
			}
		}
		if hasDefaultPermission(user.Role.Name, resource, action) {
			return true, nil
		}
	}

	return false, nil
}
