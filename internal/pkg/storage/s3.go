package storage

import (
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
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

// allowedMIMETypes maps permitted MIME types to their canonical file extension.
var allowedMIMETypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// StorageClient defines the interface for object storage operations.
type StorageClient interface {
	// UploadLogo uploads a logo file and returns the public URL and object key.
	// Returns a 400 error for invalid file type/size, 500 for upload failures.
	UploadLogo(file multipart.File, header *multipart.FileHeader) (url string, key string, apiErr *response.Error)
	// UploadAvatar uploads a user avatar file and returns the public URL and object key.
	// Returns a 400 error for invalid file type/size, 500 for upload failures.
	UploadAvatar(file multipart.File, header *multipart.FileHeader) (url string, key string, apiErr *response.Error)
	// DeleteObject removes an uploaded object by key (used for orphan cleanup).
	DeleteObject(ctx context.Context, key string) error
	// UploadAttachment uploads a task attachment and returns the public URL, object key, sanitized filename, and detected MIME type.
	UploadAttachment(file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, maxSizeMB int64) (url string, key string, sanitizedName string, mimeType string, apiErr *response.Error)
	// UploadCommentAttachment uploads a comment attachment and returns the public URL, object key, sanitized filename, and detected MIME type.
	UploadCommentAttachment(file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, maxSizeMB int64) (url string, key string, sanitizedName string, mimeType string, apiErr *response.Error)
	// GetObject retrieves an object from S3.
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error)
}

type s3Client struct {
	client         *s3.Client
	bucket         string
	endpoint       string
	publicEndpoint string
	maxSizeMB      int64
	logger         *zap.Logger
}

// NewS3Client builds an S3-compatible client pointed at the Supabase S3 endpoint.
// Configuration is read from environment variables:
//   - S3_ENDPOINT           (e.g. https://….storage.supabase.co/storage/v1/s3)
//   - S3_PUBLIC_ENDPOINT    (optional; e.g. https://….supabase.co/storage/v1/object/public)
//   - S3_REGION             (e.g. ap-south-1)
//   - S3_ACCESS_KEY_ID
//   - S3_SECRET_ACCESS_KEY
//   - S3_BUCKET             (e.g. work_pilot_bucket)
//   - S3_MAX_FILE_SIZE_MB   (default: 5)
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
		// Supabase S3 requires path-style access (not virtual-hosted).
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

// uploadImage handles common validation, unique key generation, S3 upload, and public URL construction.
func (c *s3Client) uploadImage(folder string, file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	// 1. Enforce maximum file size.
	maxBytes := c.maxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		return "", "", &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", c.maxSizeMB),
		}
	}

	// 2. Read file bytes for MIME detection and upload.
	fileBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		c.logger.Error("Failed to read uploaded file", zap.Error(readErr))
		return "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to process uploaded file. Please try again.",
		}
	}

	// 3. Detect MIME type from first 512 bytes (ignores client Content-Type).
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

	// 4. Generate a unique object key.
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

	// 5. Upload to S3.
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

	// 6. Build the public URL using the Supabase object public endpoint.
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
		c.logger.Error("Failed to delete orphaned logo from S3", zap.Error(err), zap.String("key", key))
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

	// Keep only alphanumeric characters, underscore, hyphen, and dot
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := reg.ReplaceAllString(nameWithoutExt, "_")

	// Collapse multiple underscores
	regMultiple := regexp.MustCompile(`_+`)
	sanitized = regMultiple.ReplaceAllString(sanitized, "_")

	// Trim underscores/hyphens from ends
	sanitized = strings.Trim(sanitized, "_-")

	if sanitized == "" {
		sanitized = "attachment"
	}

	// Limit length
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized + strings.ToLower(ext)
}

func (c *s3Client) UploadAttachment(file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, maxSizeMB int64) (string, string, string, string, *response.Error) {
	// 1. Enforce maximum file size.
	maxBytes := maxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", maxSizeMB),
		}
	}

	// 2. Validate file type by extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedAttachmentExtensions[ext] {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Unsupported file type. Only PNG, JPG/JPEG, PDF, DOCX, XLSX, and ZIP files are accepted.",
		}
	}

	// 3. Read file bytes
	fileBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		c.logger.Error("Failed to read uploaded file", zap.Error(readErr))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to process uploaded file. Please try again.",
		}
	}

	// 4. Validate content type
	detectedMIME := http.DetectContentType(fileBytes[:min(512, len(fileBytes))])
	mimeBase := strings.Split(detectedMIME, ";")[0]
	mimeBase = strings.TrimSpace(strings.ToLower(mimeBase))

	clientMIME := header.Header.Get("Content-Type")
	finalMIME := mimeBase

	isValid := false
	switch ext {
	case ".png":
		isValid = mimeBase == "image/png"
	case ".jpg", ".jpeg":
		isValid = mimeBase == "image/jpeg"
	case ".pdf":
		isValid = mimeBase == "application/pdf"
	case ".docx":
		isValid = mimeBase == "application/zip" || mimeBase == "application/octet-stream" || clientMIME == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		finalMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		isValid = mimeBase == "application/zip" || mimeBase == "application/octet-stream" || clientMIME == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		finalMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		isValid = mimeBase == "application/zip" || mimeBase == "application/x-zip-compressed" || mimeBase == "application/octet-stream" || clientMIME == "application/zip" || clientMIME == "application/x-zip-compressed"
		finalMIME = "application/zip"
	}

	if !isValid {
		if clientMIME != "" && (strings.Contains(clientMIME, ext[1:]) || clientMIME == "application/octet-stream") {
			isValid = true
			finalMIME = clientMIME
		}
	}

	if !isValid {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Unsupported file content type.",
		}
	}

	// 5. Sanitize Filename
	sanitizedName := SanitizeFilename(header.Filename)

	// 6. Generate unique key
	id, uuidErr := uuid.NewV7()
	if uuidErr != nil {
		c.logger.Error("Failed to generate UUID for file key", zap.Error(uuidErr))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	key := fmt.Sprintf("tasks/%s/attachments/%s-%s", taskID.String(), id.String(), sanitizedName)

	// 7. Upload to S3
	_, putErr := c.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(fileBytes),
		ContentType:   aws.String(finalMIME),
		ContentLength: aws.Int64(int64(len(fileBytes))),
	})
	if putErr != nil {
		c.logger.Error("Failed to upload attachment to S3", zap.Error(putErr), zap.String("key", key))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to upload file. Please try again later.",
		}
	}

	cleanEndpoint := strings.TrimRight(c.publicEndpoint, "/")
	publicURL := fmt.Sprintf("%s/%s/%s", cleanEndpoint, c.bucket, key)

	c.logger.Info("Attachment uploaded successfully", zap.String("key", key), zap.String("url", publicURL))
	return publicURL, key, sanitizedName, finalMIME, nil
}

func (c *s3Client) UploadCommentAttachment(file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, maxSizeMB int64) (string, string, string, string, *response.Error) {
	// 1. Enforce maximum file size.
	maxBytes := maxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("File exceeds the maximum allowed size of %d MB.", maxSizeMB),
		}
	}

	// 2. Validate file type by extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedAttachmentExtensions[ext] {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Unsupported file type. Only PNG, JPG/JPEG, PDF, DOCX, XLSX, and ZIP files are accepted.",
		}
	}

	// 3. Read file bytes
	fileBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		c.logger.Error("Failed to read uploaded file", zap.Error(readErr))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to process uploaded file. Please try again.",
		}
	}

	// 4. Validate content type
	detectedMIME := http.DetectContentType(fileBytes[:min(512, len(fileBytes))])
	mimeBase := strings.Split(detectedMIME, ";")[0]
	mimeBase = strings.TrimSpace(strings.ToLower(mimeBase))

	clientMIME := header.Header.Get("Content-Type")
	finalMIME := mimeBase

	isValid := false
	switch ext {
	case ".png":
		isValid = mimeBase == "image/png"
	case ".jpg", ".jpeg":
		isValid = mimeBase == "image/jpeg"
	case ".pdf":
		isValid = mimeBase == "application/pdf"
	case ".docx":
		isValid = mimeBase == "application/zip" || mimeBase == "application/octet-stream" || clientMIME == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		finalMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		isValid = mimeBase == "application/zip" || mimeBase == "application/octet-stream" || clientMIME == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		finalMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		isValid = mimeBase == "application/zip" || mimeBase == "application/x-zip-compressed" || mimeBase == "application/octet-stream" || clientMIME == "application/zip" || clientMIME == "application/x-zip-compressed"
		finalMIME = "application/zip"
	}

	if !isValid {
		if clientMIME != "" && (strings.Contains(clientMIME, ext[1:]) || clientMIME == "application/octet-stream") {
			isValid = true
			finalMIME = clientMIME
		}
	}

	if !isValid {
		return "", "", "", "", &response.Error{
			Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Unsupported file content type.",
		}
	}

	// 5. Sanitize Filename
	sanitizedName := SanitizeFilename(header.Filename)

	// 6. Generate unique key
	id, uuidErr := uuid.NewV7()
	if uuidErr != nil {
		c.logger.Error("Failed to generate UUID for file key", zap.Error(uuidErr))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	key := fmt.Sprintf("comments/%s/attachments/%s-%s", commentID.String(), id.String(), sanitizedName)

	// 7. Upload to S3
	_, putErr := c.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(fileBytes),
		ContentType:   aws.String(finalMIME),
		ContentLength: aws.Int64(int64(len(fileBytes))),
	})
	if putErr != nil {
		c.logger.Error("Failed to upload attachment to S3", zap.Error(putErr), zap.String("key", key))
		return "", "", "", "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to upload file. Please try again later.",
		}
	}

	cleanEndpoint := strings.TrimRight(c.publicEndpoint, "/")
	publicURL := fmt.Sprintf("%s/%s/%s", cleanEndpoint, c.bucket, key)

	c.logger.Info("Attachment uploaded successfully", zap.String("key", key), zap.String("url", publicURL))
	return publicURL, key, sanitizedName, finalMIME, nil
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
	return out.Body, *out.ContentLength, nil
}
