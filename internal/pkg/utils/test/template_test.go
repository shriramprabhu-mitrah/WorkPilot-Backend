package utils_test

import (
	"strings"
	"testing"

	"github.com/ms-kanban-server/internal/pkg/utils"
)

func TestRenderEmbeddedTemplateRendersPasswordResetTemplate(t *testing.T) {
	rendered, err := utils.RenderEmbeddedTemplate("password_reset.html", map[string]any{"OTP": "123456", "ExpiryMinutes": 15})
	if err != nil {
		t.Fatalf("expected embedded template to render, got error: %v", err)
	}

	if !strings.Contains(rendered, "123456") {
		t.Fatalf("expected rendered template to include OTP, got %s", rendered)
	}

	if !strings.Contains(rendered, "15") {
		t.Fatalf("expected rendered template to include expiry minutes, got %s", rendered)
	}
}

func TestRenderEmbeddedTemplateErrorBranches(t *testing.T) {
	t.Run("rejects empty template name", func(t *testing.T) {
		_, err := utils.RenderEmbeddedTemplate("", map[string]string{"OTP": "123456"})
		if err == nil {
			t.Fatal("expected empty template name to return an error")
		}
	})

	t.Run("returns error for missing template", func(t *testing.T) {
		_, err := utils.RenderEmbeddedTemplate("does_not_exist.html", nil)
		if err == nil {
			t.Fatal("expected missing template to return an error")
		}
	})
}
