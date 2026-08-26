package response

import (
	"strings"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/utils"
)

func OrganizationFromModel(org models.Organization, projectCount int, memberCount int) OrganizationSummary {
	return OrganizationSummary{
		ID:            org.ID,
		Name:          org.Name,
		Slug:          org.Slug,
		Domain:        org.Domain,
		Industry:      org.Industry,
		TeamSize:      org.TeamSize,
		Country:       org.Country,
		LogoURL:       org.LogoURL,
		IsActive:      org.IsActive,
		CreatedAt:     org.CreatedAt,
		TotalProjects: projectCount,
		TotalMembers:  memberCount,
	}
}

func UserProfileFromModel(user models.User) UserProfile {
	var avatarURL *string
	if user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}
	return UserProfile{
		ID:                    user.ID,
		OrganizationID:        user.OrganizationID,
		OrganizationName:      user.Organization.Name,
		Name:                  user.FullName,
		Username:              user.UserName,
		Email:                 user.Email,
		Role:                  user.Role.Name,
		AvatarURL:             avatarURL,
		Color:                 user.Color,
		Timezone:              user.Timezone,
		IsActive:              user.IsActive,
		IsVerified:            user.IsVerified,
		Status:                user.Status,
		CreatedAt:             user.CreatedAt,
		JoinedAt:              user.JoinedAt,
		RequirePasswordChange: user.RequirePasswordChange,
	}
}

func ProjectSummaryFromModel(project models.Project, taskCount int, memberCount int) ProjectSummary {
	sprints := make([]Sprint, 0, len(project.Sprints))
	for _, sprint := range project.Sprints {
		sprints = append(sprints, SprintFromModel(sprint))
	}

	key := utils.GenerateProjectPrefix(project.Name)

	return ProjectSummary{
		ID:               project.ID,
		OrganizationID:   project.OrganizationID,
		OrganizationName: project.Organization.Name,
		Name:             project.Name,
		Key:              key,
		ProjectKey:       key,
		Description:      project.Description,
		Status:           project.Status,
		CreatedBy:        project.CreatedBy,
		CreatedAt:        project.CreatedAt,
		SprintCount:      project.SprintCount,
		TotalTasks:       taskCount,
		TotalMembers:     memberCount,
		Sprints:          sprints,
		Slug:             project.Slug,
	}
}

func ProjectMemberFromModel(member models.ProjectMember) ProjectMember {
	var avatarURL *string
	if member.User.AvatarURL != "" {
		avatarURL = &member.User.AvatarURL
	}
	orgName := member.User.Organization.Name
	if orgName == "" {
		orgName = member.Project.Organization.Name
	}
	return ProjectMember{
		UserID:           member.UserID,
		Username:         member.User.UserName,
		FullName:         member.User.FullName,
		Role:             member.Role.Name,
		AvatarURL:        avatarURL,
		Color:            member.User.Color,
		OrganizationName: orgName,
		ProjectKey:       utils.GenerateProjectPrefix(member.Project.Name),
	}
}

func SprintFromModel(sprint models.Sprint) Sprint {
	return Sprint{
		ID:        sprint.ID,
		Name:      sprint.Name,
		Goal:      sprint.Goal,
		Status:    sprint.Status,
		StartDate: sprint.StartDate,
		EndDate:   sprint.EndDate,
	}
}

func CommentsFromModel(comment models.Comments) CommentsResponse {
	var attachments []CommentAttachmentResponse
	for _, a := range comment.Attachments {
		attachments = append(attachments, CommentAttachmentFromModel(a))
	}

	var avatarURL *string
	if comment.User.AvatarURL != "" {
		avatarURL = &comment.User.AvatarURL
	}

	resp := CommentsResponse{
		ID:              comment.ID,
		TaskID:          comment.TaskID,
		UserStoryID:     comment.UserStoryID,
		UserID:          comment.UserID,
		UserName:        comment.User.UserName,
		FullName:        comment.User.FullName,
		Email:           comment.User.Email,
		AvatarURL:       avatarURL,
		Color:           comment.User.Color,
		Content:         comment.Content,
		ParentCommentID: comment.ParentCommentID,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
		IsDeleted:       comment.IsDeleted,
		Attachments:     attachments,
		RepliesCount:    comment.RepliesCount,
	}

	if comment.ParentComment != nil {
		var parentAvatarURL *string
		if comment.ParentComment.User.AvatarURL != "" {
			parentAvatarURL = &comment.ParentComment.User.AvatarURL
		}
		resp.ParentComment = &ParentUserResponse{
			ID:        comment.ParentComment.ID,
			UserID:    comment.ParentComment.UserID,
			UserName:  comment.ParentComment.User.UserName,
			FullName:  comment.ParentComment.User.FullName,
			Email:     comment.ParentComment.User.Email,
			AvatarURL: parentAvatarURL,
			Color:     comment.ParentComment.User.Color,
			Content:   comment.ParentComment.Content,
			CreatedAt: comment.ParentComment.CreatedAt,
			UpdatedAt: comment.ParentComment.UpdatedAt,
			IsDeleted: comment.ParentComment.IsDeleted,
		}
	}

	return resp
}

func AuditLogFromModel(audit models.AuditLog) AuditLogResponse {
	resp := AuditLogResponse{
		ID:             audit.ID,
		ProjectID:      audit.ProjectID,
		ProjectName:    audit.ProjectName,
		OrganizationID: audit.OrganizationID,
		Action:         audit.Action,
		ResourceType:   audit.ResourceType,
		ResourceID:     audit.ResourceID,
		Details:        audit.Details,
		CreatedAt:      audit.CreatedAt,
		Title:          audit.Title,
		TaskName:       audit.TaskName,
		UserStoryName:  audit.UserStoryName,
		SprintName:     audit.SprintName,
		TaskID:         audit.TaskID,
		UserStoryID:    audit.UserStoryID,
		TaskKey:        audit.TaskKey,
		Type:           audit.Type,
	}

	if audit.User.ID != uuid.Nil {
		avatarURL := &audit.User.AvatarURL
		resp.User = &UserSummary{
			ID:        audit.User.ID,
			FullName:  audit.User.FullName,
			Email:     audit.User.Email,
			AvatarURL: avatarURL,
			Color:     audit.User.Color,
			Role:      audit.User.Role.Name,
		}
	}

	if resp.TaskKey == "" && strings.EqualFold(audit.ResourceType, "task") {
		resp.TaskKey = audit.TaskKey
	}

	return resp
}
