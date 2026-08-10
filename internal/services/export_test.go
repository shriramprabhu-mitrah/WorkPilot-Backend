package services

import "context"

// ProcessOrphanedFilesForTest exposes the unexported processOrphanedFiles method for tests.
func (s *attachmentService) ProcessOrphanedFilesForTest(ctx context.Context) {
	s.processOrphanedFiles(ctx)
}
