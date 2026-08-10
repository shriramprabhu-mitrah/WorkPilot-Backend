package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

// allowedMIMETypes maps permitted MIME types to their canonical file extension.
var allowedMIMETypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// s3API encapsulates S3 SDK client operations for unit testing.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// StorageClient defines the interface for object storage operations.
type StorageClient interface {
	UploadLogo(file multipart.File, header *multipart.FileHeader) (url string, key string, apiErr *response.Error)
	UploadAvatar(file multipart.File, header *multipart.FileHeader) (url string, key string, apiErr *response.Error)
	DeleteObject(ctx context.Context, key string) error
	UploadAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (key string, sanitizedName string, mimeType string, apiErr *response.Error)
	UploadCommentAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, cfg models.AttachmentConfig) (key string, sanitizedName string, mimeType string, apiErr *response.Error)
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error)
}

type s3Client struct {
	client         s3API
	bucket         string
	endpoint       string
	publicEndpoint string
	maxSizeMB      int64
	logger         *zap.Logger
}

// NewS3Client builds an S3-compatible client pointed at the Supabase S3 endpoint.
func NewS3Client(logger *zap.Logger) StorageClient {
	endpoint := config.GetEnv("S3_ENDPOINT", "")
	publicEndpoint := config.GetEnv("S3_PUBLIC_ENDPOINT", "")
	region := config.GetEnv("S3_REGION", "ap-south-1")
	accessKey := config.GetEnv("S3_ACCESS_KEY_ID", "")
	secretKey := config.GetEnv("S3_SECRET_ACCESS_KEY", "")
	bucket := config.GetEnv("S3_BUCKET", "work_pilot_bucket")

	maxSizeMB := int64(5)
	if v := config.GetEnv("S3_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	client := s3.New(s3.Options{
		Region:       region,
		Credentials:  creds,
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
	})

	if publicEndpoint == "" {
		publicEndpoint = derivePublicEndpoint(endpoint)
	}

	return &s3Client{
		client:         client,
		bucket:         bucket,
		endpoint:       endpoint,
		publicEndpoint: publicEndpoint,
		maxSizeMB:      maxSizeMB,
		logger:         logger,
	}
}

// UploadLogo validates, uploads a logo file to S3 and returns its public URL and object key.
func (c *s3Client) UploadLogo(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return c.uploadImage("organizations/logos", file, header)
}

// UploadAvatar validates, uploads a user avatar file to S3 and returns its public URL and object key.
func (c *s3Client) UploadAvatar(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return c.uploadImage("users/avatars", file, header)
}

func (c *s3Client) uploadImage(folder string, file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	maxBytes := c.maxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		return "", "", &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", c.maxSizeMB),
		}
	}

	fileBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		c.logger.Error("Failed to read uploaded file", zap.Error(readErr))
		return "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to process uploaded file. Please try again.",
		}
	}

	detectedMIME := http.DetectContentType(fileBytes[:min(512, len(fileBytes))])
	mimeBase := strings.Split(detectedMIME, ";")[0]
	mimeBase = strings.TrimSpace(strings.ToLower(mimeBase))

	ext, ok := allowedMIMETypes[mimeBase]
	if !ok {
		return "", "", &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid file type. Only PNG, JPG/JPEG, and WEBP images are accepted.",
		}
	}

	id, uuidErr := uuid.NewV7()
	if uuidErr != nil {
		c.logger.Error("Failed to generate UUID for file key", zap.Error(uuidErr))
		return "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	key := fmt.Sprintf("%s/%s.%s", folder, id.String(), ext)

	_, putErr := c.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(fileBytes),
		ContentType:   aws.String(mimeBase),
		ContentLength: aws.Int64(int64(len(fileBytes))),
	})
	if putErr != nil {
		c.logger.Error("Failed to upload file to S3", zap.Error(putErr), zap.String("key", key))
		return "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to upload file. Please try again later.",
		}
	}

	cleanEndpoint := strings.TrimRight(c.publicEndpoint, "/")
	publicURL := fmt.Sprintf("%s/%s/%s", cleanEndpoint, c.bucket, key)

	c.logger.Info("File uploaded successfully", zap.String("key", key), zap.String("url", publicURL))
	return publicURL, key, nil
}

// DeleteObject removes an object from S3 by key. Used for orphan cleanup.
func (c *s3Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.logger.Error("Failed to delete object from S3", zap.Error(err), zap.String("key", key))
	}
	return err
}

func derivePublicEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}

	cleanEndpoint := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(cleanEndpoint, "/s3") {
		return strings.TrimSuffix(cleanEndpoint, "/s3") + "/object/public"
	}
	if strings.Contains(cleanEndpoint, "/s3/") {
		return strings.Replace(cleanEndpoint, "/s3/", "/object/public/", 1)
	}
	if strings.Contains(cleanEndpoint, "/object/public") {
		return cleanEndpoint
	}
	return cleanEndpoint + "/object/public"
}

var allowedAttachmentExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".pdf":  true,
	".docx": true,
	".xlsx": true,
	".zip":  true,
}

func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := reg.ReplaceAllString(nameWithoutExt, "_")

	regMultiple := regexp.MustCompile(`_+`)
	sanitized = regMultiple.ReplaceAllString(sanitized, "_")

	sanitized = strings.Trim(sanitized, "_-")

	if sanitized == "" {
		sanitized = "attachment"
	}

	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized + strings.ToLower(ext)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *s3Client) uploadAttachmentStream(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	maxBytes := cfg.MaxFileSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		return "", "", "", &response.Error{
			Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", cfg.MaxFileSizeMB),
		}
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedAttachmentExtensions[ext] {
		return "", "", "", &response.Error{
			Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Unsupported file type. Only PNG, JPG/JPEG, PDF, DOCX, XLSX, and ZIP files are accepted.",
		}
	}

	var bodyReader io.Reader
	var finalMIME string
	var finalSize int64
	isValid := false

	// Validate archives (ZIP, DOCX, XLSX) against ZIP bombs, traversal, and structural requirements.
	// Note: The DOCX/XLSX validation is structural-only (confirming key entry files exist to prevent renamed executables)
	// and does not guarantee semantic validity or parsing correctness of the document.
	if ext == ".docx" || ext == ".xlsx" || ext == ".zip" {
		fileBytes, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
		if readErr != nil {
			c.logger.Error("Failed to read archive file", zap.Error(readErr))
			return "", "", "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to process uploaded archive.",
			}
		}

		if int64(len(fileBytes)) > maxBytes {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
				StatusCode: http.StatusRequestEntityTooLarge,
				Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", cfg.MaxFileSizeMB),
			}
		}

		// Sniff MIME type
		sniffLen := min(512, len(fileBytes))
		detectedMIME := http.DetectContentType(fileBytes[:sniffLen])
		mimeBase := strings.Split(detectedMIME, ";")[0]
		mimeBase = strings.TrimSpace(strings.ToLower(mimeBase))

		if mimeBase != "application/zip" {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    "Invalid file format. File is not a valid ZIP container.",
			}
		}

		zipReader, zipErr := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
		if zipErr != nil {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    "Invalid zip file structure.",
			}
		}

		const maxEntries = 1000
		const maxRatio = 100
		maxUncompressedTotal := int64(50 * 1024 * 1024) // 50MB uncompressed limit

		if len(zipReader.File) > maxEntries {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    "ZIP archive contains too many files.",
			}
		}

		var totalUncompressedSize int64
		hasContentTypes := false
		hasWordDoc := false
		hasWorkbook := false

		for _, f := range zipReader.File {
			cleanedPath := filepath.Clean(f.Name)
			if strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") || strings.HasPrefix(cleanedPath, "..") || hasDotDotComponent(f.Name) {
				return "", "", "", &response.Error{
					Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
					StatusCode: http.StatusUnsupportedMediaType,
					Message:    "ZIP archive contains unsafe file paths.",
				}
			}

			totalUncompressedSize += int64(f.UncompressedSize64)
			if totalUncompressedSize > maxUncompressedTotal {
				return "", "", "", &response.Error{
					Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
					StatusCode: http.StatusRequestEntityTooLarge,
					Message:    "ZIP archive uncompressed size exceeds maximum limit.",
				}
			}

			if f.CompressedSize64 > 0 {
				ratio := float64(f.UncompressedSize64) / float64(f.CompressedSize64)
				if ratio > maxRatio {
					return "", "", "", &response.Error{
						Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
						StatusCode: http.StatusUnsupportedMediaType,
						Message:    "ZIP archive contains excessively compressed files (potential zip-bomb).",
					}
				}
			}

			if f.Name == "[Content_Types].xml" {
				hasContentTypes = true
			}
			if f.Name == "word/document.xml" {
				hasWordDoc = true
			}
			if f.Name == "xl/workbook.xml" {
				hasWorkbook = true
			}
		}

		if ext == ".docx" {
			isValid = hasContentTypes && hasWordDoc
			finalMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		} else if ext == ".xlsx" {
			isValid = hasContentTypes && hasWorkbook
			finalMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else if ext == ".zip" {
			isValid = true
			finalMIME = "application/zip"
		}

		if !isValid {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    fmt.Sprintf("Invalid OOXML document structure for %s extension.", ext),
			}
		}

		bodyReader = bytes.NewReader(fileBytes)
		finalSize = int64(len(fileBytes))
	} else {
		// Images and PDF validation (streaming, read first 512 bytes)
		sniffBuf := make([]byte, 512)
		n, readErr := io.ReadFull(file, sniffBuf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			c.logger.Error("Failed to read uploaded file header", zap.Error(readErr))
			return "", "", "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to process uploaded file header. Please try again.",
			}
		}
		sniffBuf = sniffBuf[:n]

		detectedMIME := http.DetectContentType(sniffBuf)
		mimeBase := strings.Split(detectedMIME, ";")[0]
		mimeBase = strings.TrimSpace(strings.ToLower(mimeBase))

		switch ext {
		case ".png":
			isValid = mimeBase == "image/png"
			finalMIME = "image/png"
		case ".jpg", ".jpeg":
			isValid = mimeBase == "image/jpeg"
			finalMIME = "image/jpeg"
		case ".pdf":
			isValid = mimeBase == "application/pdf"
			finalMIME = "application/pdf"
		}

		if !isValid {
			return "", "", "", &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    "Unsupported file content type.",
			}
		}

		limitedReader := io.LimitReader(file, maxBytes-int64(n))
		bodyReader = io.MultiReader(bytes.NewReader(sniffBuf), limitedReader)
		finalSize = header.Size
	}

	sanitizedName := SanitizeFilename(header.Filename)

	id, uuidErr := uuid.NewV7()
	if uuidErr != nil {
		c.logger.Error("Failed to generate UUID for file key", zap.Error(uuidErr))
		return "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	key := fmt.Sprintf("%s/%s-%s", folder, id.String(), sanitizedName)

	_, putErr := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bodyReader,
		ContentType:   aws.String(finalMIME),
		ContentLength: aws.Int64(finalSize),
	})
	if putErr != nil {
		c.logger.Error("Failed to upload attachment to S3", zap.Error(putErr), zap.String("key", key))
		return "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to upload file. Please try again later.",
		}
	}

	c.logger.Info("Attachment uploaded successfully to storage", zap.String("key", key))
	return key, sanitizedName, finalMIME, nil
}

func (c *s3Client) UploadAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	folder := fmt.Sprintf("tasks/%s/attachments", taskID.String())
	return c.uploadAttachmentStream(ctx, file, header, folder, cfg)
}

func (c *s3Client) UploadCommentAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	folder := fmt.Sprintf("comments/%s/attachments", commentID.String())
	return c.uploadAttachmentStream(ctx, file, header, folder, cfg)
}

func (c *s3Client) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.logger.Error("Failed to get file from S3", zap.Error(err), zap.String("key", key))
		return nil, 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve file from storage.",
		}
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, nil
}

func hasDotDotComponent(path string) bool {
	// Normalize backslashes to forward slashes for ZIP entries
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}
