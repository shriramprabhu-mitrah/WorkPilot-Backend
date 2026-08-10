package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
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

type mockS3API struct {
	putObjectCalls    int
	deleteObjectCalls int
	getObjectCalls    int
}

func (m *mockS3API) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putObjectCalls++
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3API) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteObjectCalls++
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3API) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getObjectCalls++
	return &s3.GetObjectOutput{}, nil
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
	mockAPI := &mockS3API{}
	c := &s3Client{
		client: mockAPI,
		bucket: "test-bucket",
		logger: zap.NewNop(),
	}

	cfg := models.AttachmentConfig{
		MaxFileSizeMB: 10,
		MaxFiles:      5,
	}

	runValidation := func(filename string, content []byte, shouldPass bool, expectedErr string) {
		mockAPI.putObjectCalls = 0

		file := newMockMultipartFile(content)
		header := &multipart.FileHeader{
			Filename: filename,
			Size:     int64(len(content)),
		}

		_, _, _, err := c.UploadAttachment(context.Background(), file, header, uuid.Nil, cfg)
		if shouldPass {
			if err != nil {
				t.Errorf("Expected %s to pass validation, but got error: %v", filename, err)
			}
			if mockAPI.putObjectCalls != 1 {
				t.Errorf("Expected PutObject to be called exactly once for %s, got %d", filename, mockAPI.putObjectCalls)
			}
		} else {
			if err == nil {
				t.Errorf("Expected %s to fail validation, but it succeeded", filename)
			} else if !strings.Contains(string(err.Code), expectedErr) && !strings.Contains(err.Message, expectedErr) {
				t.Errorf("Expected error for %s to contain %s, got code=%s msg=%s", filename, expectedErr, err.Code, err.Message)
			}
			if mockAPI.putObjectCalls != 0 {
				t.Errorf("Expected PutObject to never be called for %s, got %d", filename, mockAPI.putObjectCalls)
			}
		}
	}

	// 1. Valid cases
	runValidation("image.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01"), true, "")
	runValidation("doc.pdf", []byte("%PDF-1.4\n%..."), true, "")

	// Valid zip archive structure for DOCX/XLSX/ZIP
	// DOCX needs [Content_Types].xml and word/document.xml
	validDocx := new(bytes.Buffer)
	zw := zip.NewWriter(validDocx)
	_, _ = zw.Create("[Content_Types].xml")
	_, _ = zw.Create("word/document.xml")
	_ = zw.Close()
	runValidation("document.docx", validDocx.Bytes(), true, "")

	// XLSX needs [Content_Types].xml and xl/workbook.xml
	validXlsx := new(bytes.Buffer)
	zw2 := zip.NewWriter(validXlsx)
	_, _ = zw2.Create("[Content_Types].xml")
	_, _ = zw2.Create("xl/workbook.xml")
	_ = zw2.Close()
	runValidation("spreadsheet.xlsx", validXlsx.Bytes(), true, "")

	// Generic zip archive
	validZip := new(bytes.Buffer)
	zw3 := zip.NewWriter(validZip)
	_, _ = zw3.Create("somefile.txt")
	_ = zw3.Close()
	runValidation("archive.zip", validZip.Bytes(), true, "")

	// 2. Invalid cases
	// Renamed malicious file (.exe renamed to .pdf)
	runValidation("malicious.pdf", []byte("MZ\x90\x00\x03\x00\x00\x00"), false, "UNSUPPORTED_MEDIA_TYPE")

	// Invalid zip renaming (a text file renamed to .docx)
	runValidation("bad.docx", []byte("this is not a zip"), false, "UNSUPPORTED_MEDIA_TYPE")

	// Empty file
	runValidation("empty.png", []byte(""), false, "UNSUPPORTED_MEDIA_TYPE")

	// Path traversal in ZIP archive entry
	traversalZip := new(bytes.Buffer)
	zw4 := zip.NewWriter(traversalZip)
	_, _ = zw4.Create("../unsafe.txt")
	_ = zw4.Close()
	runValidation("traversal.zip", traversalZip.Bytes(), false, "UNSUPPORTED_MEDIA_TYPE")

	// Over uncompressed size limit zip-bomb test
	// Create a zip with 1 entry exceeding uncompressed limits
	bombZip := new(bytes.Buffer)
	zw5 := zip.NewWriter(bombZip)
	h := &zip.FileHeader{
		Name:   "huge.txt",
		Method: zip.Deflate,
	}
	w, _ := zw5.CreateHeader(h)
	// Write 51MB of zeroes in chunks of 32KB to avoid memory usage
	chunk := make([]byte, 32*1024)
	for i := 0; i < 1633; i++ {
		_, _ = w.Write(chunk)
	}
	_ = zw5.Close()
	
	runValidation("bomb.zip", bombZip.Bytes(), false, "PAYLOAD_TOO_LARGE")

	// Size limit check
	runValidation("huge.png", make([]byte, 11*1024*1024), false, "PAYLOAD_TOO_LARGE")
}
