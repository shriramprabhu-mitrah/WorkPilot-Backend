package storage

import (
	"bytes"
	"context"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

type mockMultipartFile struct {
	*bytes.Reader
}

func (m *mockMultipartFile) Close() error {
	return nil
}

func newMockMultipartFile(data []byte) multipart.File {
	return &mockMultipartFile{
		Reader: bytes.NewReader(data),
	}
}

func TestDerivePublicEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "path style s3 endpoint",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/s3",
			want:     "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/object/public",
		},
		{
			name:     "embedded s3 path",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/s3/some",
			want:     "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/object/public/some",
		},
		{
			name:     "already public object endpoint",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.supabase.co/storage/v1/object/public",
			want:     "https://rywvrcvpgeenhvlyrtaj.supabase.co/storage/v1/object/public",
		},
		{
			name:     "missing s3 segment",
			endpoint: "https://example.com/base",
			want:     "https://example.com/base/object/public",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := derivePublicEndpoint(tt.endpoint)
			if got != tt.want {
				t.Fatalf("derivePublicEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestUploadAttachmentValidation(t *testing.T) {
	c := &s3Client{
		client: nil, // If validation passes, it will panic on S3 PutObject call
		bucket: "test-bucket",
		logger: zap.NewNop(),
	}

	runValidation := func(filename string, content []byte, shouldPass bool, expectedErr string) {
		file := newMockMultipartFile(content)
		header := &multipart.FileHeader{
			Filename: filename,
			Size:     int64(len(content)),
		}

		defer func() {
			r := recover()
			if r != nil {
				if !shouldPass {
					t.Errorf("Validation for %s should have failed, but it passed and panicked on S3 client call: %v", filename, r)
				}
			}
		}()

		_, _, _, err := c.UploadAttachment(context.Background(), file, header, uuid.Nil, 10)
		if shouldPass {
			// If it passed validation, it should have panicked since c.client is nil.
			// If it did not panic and returned an error, check if that's expected.
			if err != nil {
				t.Errorf("Expected %s to pass validation, but got error: %v", filename, err)
			}
		} else {
			if err == nil {
				t.Errorf("Expected %s to fail validation, but it succeeded", filename)
			} else if !strings.Contains(string(err.Code), expectedErr) && !strings.Contains(err.Message, expectedErr) {
				t.Errorf("Expected error for %s to contain %s, got code=%s msg=%s", filename, expectedErr, err.Code, err.Message)
			}
		}
	}

	// 1. Valid cases (should pass validation and trigger PutObject panic)
	runValidation("image.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01"), true, "")
	runValidation("doc.pdf", []byte("%PDF-1.4\n%..."), true, "")

	// DOCX, XLSX and ZIP files all sniff as "application/zip" (PK\x03\x04 header)
	zipHeader := []byte("PK\x03\x04\x14\x00\x08\x00\x08\x00")
	runValidation("archive.zip", zipHeader, true, "")
	runValidation("document.docx", zipHeader, true, "")
	runValidation("spreadsheet.xlsx", zipHeader, true, "")

	// 2. Invalid cases
	// Mismatched extension (.exe renamed to .pdf)
	runValidation("malicious.pdf", []byte("MZ\x90\x00\x03\x00\x00\x00"), false, "UNSUPPORTED_MEDIA_TYPE")

	// Invalid PNG header
	runValidation("bad.png", []byte("NOT_A_PNG"), false, "UNSUPPORTED_MEDIA_TYPE")

	// Unsupported extension (.exe)
	runValidation("malicious.exe", []byte("MZ\x90\x00\x03\x00\x00\x00"), false, "UNSUPPORTED_MEDIA_TYPE")

	// Empty file
	runValidation("empty.png", []byte(""), false, "UNSUPPORTED_MEDIA_TYPE")

	// Over limit
	runValidation("huge.png", make([]byte, 11*1024*1024), false, "PAYLOAD_TOO_LARGE")
}
