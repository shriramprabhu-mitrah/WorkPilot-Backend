package response

import (
	"strings"

	"github.com/ms-kanban-server/internal/pkg/models"
)

func OrganizationFromModel(org models.Organization) OrganizationSummary {
	return OrganizationSummary{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Domain:    org.Domain,
		Industry:  org.Industry,
		TeamSize:  org.TeamSize,
		Country:   org.Country,
		LogoURL:   org.LogoURL,
		IsActive:  org.IsActive,
		CreatedAt: org.CreatedAt,
	}
}

func UserProfileFromModel(user models.User) UserProfile {
	return UserProfile{
		ID:             user.ID,
		OrganizationID: user.OrganizationID,
		Name:           user.FullName,
		Username:       user.UserName,
		Email:          user.Email,
		Role:           user.Role,
		AvatarURL:      user.AvatarURL,
		Timezone:       user.Timezone,
		IsActive:       user.IsActive,
		IsVerified:     user.IsVerified,
		CreatedAt:      user.CreatedAt,
		JoinedAt:       user.JoinedAt,
	}
}

func ProjectSummaryFromModel(project models.Project) ProjectSummary {
	sprints := make([]Sprint, 0, len(project.Sprints))
	for _, sprint := range project.Sprints {
		sprints = append(sprints, SprintFromModel(sprint))
	}

	return ProjectSummary{
		ID:             project.ID,
		OrganizationID: project.OrganizationID,
		Name:           project.Name,
		Description:    project.Description,
		Status:         project.Status,
		CreatedBy:      project.CreatedBy,
		CreatedAt:      project.CreatedAt,
		SprintCount:    project.SprintCount,
		Sprints:        sprints,
	}
}

func ProjectMemberFromModel(member models.ProjectMember) ProjectMember {
	return ProjectMember{
		UserID:   member.UserID,
		Username: member.User.UserName,
		FullName: member.User.FullName,
		Role:     member.User.Role,
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

	resp := CommentsResponse{
		ID:              comment.ID,
		TaskID:          comment.TaskID,
		UserID:          comment.UserID,
		UserName:        comment.User.UserName,
		FullName:        comment.User.FullName,
		Email:           comment.User.Email,
		AvatarURL:       comment.User.AvatarURL,
		Content:         comment.Content,
		ParentCommentID: comment.ParentCommentID,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
		IsDeleted:       comment.IsDeleted,
		Attachments:     attachments,
	}

	if comment.ParentComment != nil {
		resp.ParentComment = &ParentUserResponse{
			ID:        comment.ParentComment.ID,
			UserID:    comment.ParentComment.UserID,
			UserName:  comment.ParentComment.User.UserName,
			FullName:  comment.ParentComment.User.FullName,
			Email:     comment.ParentComment.User.Email,
			AvatarURL: comment.ParentComment.User.AvatarURL,
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
		OrganizationID: audit.OrganizationID,
		Action:         audit.Action,
		ResourceType:   audit.ResourceType,
		ResourceID:     audit.ResourceID,
		Details:        audit.Details,
		CreatedAt:      audit.CreatedAt,
		Title:          audit.Title,
	}

	if strings.ToLower(audit.ResourceType) == "task" {
		resp.TaskKey = audit.TaskKey
	}

	return resp
}
