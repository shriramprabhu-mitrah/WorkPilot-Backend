package authrepo

import (
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AuthRepository interface {
	GetByEmail(email string) (models.User, *response.Error)
	GetUserByID(id uuid.UUID) (models.User, *response.Error)
	ExistsByEmail(email string) (bool, *response.Error)
	ExistsByUsername(username string) (bool, *response.Error)
	CreateUser(row models.User) *response.Error
	StoreRefreshToken(token models.RefreshToken) (models.RefreshToken, *response.Error)
	GetRefreshToken(userID string) (models.RefreshToken, *response.Error)
	GetRefreshTokenByID(id uuid.UUID) (models.RefreshToken, *response.Error)
	ChangePassword(password string, userID uuid.UUID) *response.Error
	RequestPasswordReset(email string) (models.User, *response.Error)
	SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error
	InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error
	GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error)
	SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error
	InvalidateEmailVerificationOTPs(userID uuid.UUID) *response.Error
	GetEmailVerificationOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error)
	IsEmailVerificationResendAllowed(email string, interval time.Duration) (bool, *response.Error)
	RecordEmailVerificationResend(email string, sentAt time.Time) *response.Error
	UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error
	RevokeRefreshTokens(userID uuid.UUID) *response.Error
	UpdateUser(userID uuid.UUID, req models.User) *response.Error
	StoreUserTemp(row models.User) *response.Error
	GetUserFromRedis(email string) (*models.User, *response.Error)
	GetPendingInvitationByEmail(email string) (models.OrganizationInvitation, *response.Error)
	UpdateInvitation(invitation models.OrganizationInvitation) *response.Error
	CreateAuditLog(log models.AuditLog) *response.Error
}

func InitAuthRepository(deps models.Config) AuthRepository {
	return &authDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type authDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}
