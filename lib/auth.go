package lib

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrUserNotFound = errors.New("user not found")
)

type User struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid;column:id"`
	Name         string         `gorm:"type:varchar;column:name"`
	Email        string         `gorm:"type:varchar;column:email;uniqueIndex"`
	PasswordHash string         `gorm:"type:text;column:password_hash" json:"-" swaggerignore:"true"`
	CreatedAt    time.Time      `gorm:"type:timestamptz;column:created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamptz;column:updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" swaggerignore:"true"`
}

type UserRegistration struct {
	ID        int       `gorm:"primaryKey;column:id;autoIncrement"`
	Name      string    `gorm:"type:varchar;column:name"`
	Email     string    `gorm:"type:varchar;column:email"`
	Token     string    `gorm:"type:varchar;column:token"`
	CreatedAt time.Time `gorm:"type:timestamptz;column:created_at"`
	ExpiresAt time.Time `gorm:"type:timestamptz;column:expires_at"`
}

type UserOneTimeLogin struct {
	ID        int       `gorm:"primaryKey;column:id;autoIncrement"`
	UserID    uuid.UUID `gorm:"column:user_id"`
	Token     string    `gorm:"type:varchar;column:token"`
	CreatedAt time.Time `gorm:"type:timestamptz;column:created_at"`
	ExpiresAt time.Time `gorm:"type:timestamptz;column:expires_at"`
}

type UserBearerToken struct {
	ID        int            `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	UserID    uuid.UUID      `gorm:"column:user_id" json:"user_id"`
	Token     string         `gorm:"type:varchar;column:token" json:"-" swaggerignore:"true"`
	TokenHash string         `gorm:"type:varchar(64);column:token_hash;uniqueIndex:idx_user_bearer_token_hash" json:"-" swaggerignore:"true"`
	CreatedAt time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	ExpiresAt time.Time      `gorm:"type:timestamptz;column:expires_at" json:"expires_at"`
	DeletedAt gorm.DeletedAt `json:"-" swaggerignore:"true"`
}

type WriteAccessToken struct {
	ID              uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	NameSpaceID     uuid.UUID      `gorm:"column:namespace_id" json:"namespace_id"`
	Token           string         `gorm:"-" json:"-" swaggerignore:"true"`
	TokenHash       string         `gorm:"type:varchar(64);column:token_hash;uniqueIndex:idx_write_access_token_hash" json:"-" swaggerignore:"true"`
	CreatedAt       time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	CreatedByUserID uuid.UUID      `gorm:"column:created_by_user_id" json:"created_by_user_id"`
	CreatedBy       User           `gorm:"foreignKey:CreatedByUserID" json:"created_by"`
	UpdatedAt       time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	ExpiresAt       time.Time      `gorm:"type:timestamptz;column:expires_at" json:"expires_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

type UserBearerTokenResponse struct {
	ID        int       `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type WriteAccessTokenResponse struct {
	ID          uuid.UUID `json:"id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func HashToken(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func NewWriteAccessToken(namespaceID, createdBy uuid.UUID, expiresAt time.Time) (WriteAccessToken, string, error) {
	raw, err := createRandomToken("abcdef0123456789", 32)
	if err != nil {
		return WriteAccessToken{}, "", err
	}
	now := time.Now()
	tokenID := uuid.New()
	return WriteAccessToken{ID: tokenID, NameSpaceID: namespaceID, TokenHash: HashToken(raw), CreatedAt: now, CreatedByUserID: createdBy, UpdatedAt: now, ExpiresAt: expiresAt}, raw, nil
}

func (t WriteAccessToken) Response(raw string) WriteAccessTokenResponse {
	return WriteAccessTokenResponse{ID: t.ID, NamespaceID: t.NameSpaceID, Token: raw, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt}
}

func (t UserBearerToken) Response() UserBearerTokenResponse {
	return UserBearerTokenResponse{ID: t.ID, UserID: t.UserID, Token: t.Token, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt}
}

type NamespaceAccessPermission struct {
	ID          int       `gorm:"primaryKey;column:id;autoIncrement"`
	NamespaceID uuid.UUID `gorm:"type:uuid;column:namespace_id;uniqueIndex:idx_namespace_user"`
	UserID      uuid.UUID `gorm:"type:uuid;column:user_id;uniqueIndex:idx_namespace_user"`
	GrantType   string    `gorm:"column:grant_type"` // r, w, rw, admin
	CreatedAt   time.Time `gorm:"type:timestamptz;column:created_at"`
	DeletedAt   gorm.DeletedAt
	Namespace   Namespace `gorm:"foreignKey:NamespaceID;constraint:OnDelete:RESTRICT" json:"-" swaggerignore:"true"`
	User        User      `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-" swaggerignore:"true"`
}

func (UserOneTimeLogin) TableName() string {
	return "user_one_time_login"
}

func (UserBearerToken) TableName() string {
	return "user_bearer_token"
}

func (User) TableName() string {
	return "user"
}

func (UserRegistration) TableName() string {
	return "user_registration"
}

func createRandomDigit(n int) (string, error) {
	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		val, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		bytes[i] = '0' + byte(val.Int64())
	}
	return string(bytes), nil
}

func createRandomToken(char string, n int) (string, error) {
	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		val, err := rand.Int(rand.Reader, big.NewInt(int64(len(char))))
		if err != nil {
			return "", err
		}
		bytes[i] = char[int(val.Int64())]
	}
	return string(bytes), nil
}

func (c *Controller) CreateUser(name string, email string) error {
	var userCount int64
	err := c.DB.Model(&User{}).Where("email = ?", email).Count(&userCount).Error
	if err != nil {
		return err
	}
	if userCount > 0 {
		return errors.New("user already exists")
	}

	user := User{
		ID:    uuid.New(),
		Name:  name,
		Email: email,
	}

	return c.DB.Create(&user).Error
}

func (c *Controller) CreateUserOneTimeLoginCode(userID uuid.UUID) (string, error) {
	var count int64
	err := c.DB.Model(&User{}).Where("id = ?", userID).Count(&count).Error
	if err != nil {
		return "", err
	}
	if count > 0 {
		// Only the newest code is valid. This also bounds the number of active
		// secrets kept for a user.
		if err := c.DB.Where("user_id = ?", userID).Delete(&UserOneTimeLogin{}).Error; err != nil {
			return "", err
		}
		res, err := createRandomDigit(6)
		if err != nil {
			return "", err
		}
		u := UserOneTimeLogin{
			UserID:    userID,
			Token:     HashToken(res),
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Minute * 10),
		}
		err = c.DB.Create(&u).Error
		if err != nil {
			return "", err
		}
		return res, nil
	}
	return "", ErrUserNotFound
}

func (c *Controller) CreateUserRegistrationCode(name string, email string) (string, error) {
	var userCount int64
	err := c.DB.Model(&User{}).Where("email = ?", email).Count(&userCount).Error
	if err != nil {
		return "", err
	}
	if userCount > 0 {
		return "", errors.New("user already exists")
	}

	res, err := createRandomDigit(6)
	if err != nil {
		return "", err
	}
	if err := c.DB.Where("email = ?", email).Delete(&UserRegistration{}).Error; err != nil {
		return "", err
	}
	u := UserRegistration{
		Name:      name,
		Email:     email,
		Token:     HashToken(res),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute * 30),
	}
	err = c.DB.Create(&u).Error
	if err != nil {
		return "", err
	}
	return res, nil
}

func (c *Controller) VerifyUserRegistrationCode(email string, code string) (*User, error) {
	var user User
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		var reg UserRegistration
		if err := tx.Where("email = ? AND (token = ? OR token = ?)", email, HashToken(code), code).Where("expires_at > ?", time.Now()).Order("created_at desc").First(&reg).Error; err != nil {
			return ErrInvalidToken
		}
		result := tx.Where("id = ? AND expires_at > ?", reg.ID, time.Now()).Delete(&UserRegistration{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidToken
		}
		user = User{ID: uuid.New(), Name: reg.Name, Email: reg.Email}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Where("email = ?", email).Delete(&UserRegistration{}).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Controller) GetUserOneTimeLoginCode(email string, token string) (User, error) {
	var user User
	err := c.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return User{}, err
	}

	var onetime UserOneTimeLogin
	err = c.DB.Where("token = ? OR token = ?", HashToken(token), token).Where("expires_at > ?", time.Now()).Where("user_id = ?", user.ID).First(&onetime).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrInvalidToken
	}
	if err != nil {
		return User{}, err
	}

	// Conditional deletion makes one-time use atomic even when callbacks race.
	result := c.DB.Where("id = ? AND expires_at > ?", onetime.ID, time.Now()).Delete(&UserOneTimeLogin{})
	if result.Error != nil {
		return User{}, result.Error
	}
	if result.RowsAffected != 1 {
		return User{}, ErrInvalidToken
	}

	return user, nil
}

func (c *Controller) CreateUserBearerToken(userID uuid.UUID) (UserBearerToken, error) {
	var count int64
	err := c.DB.Model(&User{}).Where("id = ?", userID).Count(&count).Error
	if err != nil {
		return UserBearerToken{}, err
	}
	if count > 0 {
		token, err := createRandomToken("abcdef0123456789", 32)
		if err != nil {
			return UserBearerToken{}, err
		}
		u := UserBearerToken{
			UserID:    userID,
			TokenHash: HashToken(token),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour * 24),
		}
		err = c.DB.Create(&u).Error
		if err != nil {
			return UserBearerToken{}, err
		}
		// Some dialects do not populate an auto-increment ID after INSERT.
		var stored UserBearerToken
		if err := c.DB.Where("token_hash = ?", u.TokenHash).First(&stored).Error; err != nil {
			return UserBearerToken{}, err
		}
		stored.Token = token // transient only; the handler converts this to a response
		return stored, nil
	}
	return UserBearerToken{}, ErrUserNotFound
}

func (c *Controller) GetUserById(id uuid.UUID) (User, error) {
	var user User
	err := c.DB.Where("id = ?", id).Where("deleted_at IS NULL").First(&user).Error
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Controller) ChangeUserNameById(userId uuid.UUID, name string) error {
	if name == "" {
		return errors.New("name is required")
	}

	var user User
	err := c.DB.Where("id = ?", userId).Where("deleted_at IS NULL").First(&user).Error
	if err != nil {
		return err
	}
	user.Name = name
	return c.DB.Save(&user).Error
}

func (c *Controller) GetUserByMailAddress(email string) (User, error) {
	var user User
	err := c.DB.Where("email = ?", email).Where("deleted_at IS NULL").First(&user).Error
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Controller) GetUserByUserBearerToken(token string) (User, error) {
	var bearer UserBearerToken
	err := c.DB.Where("token = ?", token).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL").First(&bearer).Error
	if err != nil {
		return User{}, err
	}
	var user User
	err = c.DB.Where("id = ?", bearer.UserID).First(&user).Error
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Controller) RevokeUserBearerToken(userId string) error {
	err := c.DB.Model(&UserBearerToken{}).Where("user_id = ?", userId).Where("expires_at > ?", time.Now()).Update("deleted_at", time.Now()).Error
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) PermitUserToAccessNamespace(doAsUserId string, targetUserId string, namespaceId string, grantType string) error {
	if grantType != "r" && grantType != "w" && grantType != "rw" && grantType != "admin" && grantType != "none" {
		return errors.New("invalid grant type")
	}
	err := c.DB.Model(&User{}).Where("id = ?", doAsUserId).First(&User{}).Error
	if err != nil {
		return err
	}
	var doAs NamespaceAccessPermission
	err = c.DB.Model(&NamespaceAccessPermission{}).Where("namespace_id = ?", namespaceId).Where("user_id = ?", doAsUserId).Where("deleted_at IS NULL").First(&doAs).Error
	if err != nil {
		return err
	}
	if doAs.GrantType != "admin" {
		return errors.New("doAsUser is not administrator")
	}
	err = c.DB.Model(&User{}).Where("id = ?", targetUserId).First(&User{}).Error
	if err != nil {
		return err
	}
	var target NamespaceAccessPermission
	tx := c.DB.Unscoped().Model(&NamespaceAccessPermission{}).Where("namespace_id = ?", namespaceId).Where("user_id = ?", targetUserId).First(&target)
	err = tx.Error

	if err == gorm.ErrRecordNotFound { // 新規作成
		nsID, errNs := uuid.Parse(namespaceId)
		uID, errU := uuid.Parse(targetUserId)
		if errNs != nil || errU != nil {
			return errors.New("invalid uuid format")
		}
		target = NamespaceAccessPermission{
			NamespaceID: nsID,
			UserID:      uID,
			GrantType:   grantType,
			CreatedAt:   time.Now(),
		}
		err = c.DB.Create(&target).Error
		if err != nil {
			return err
		}
		return nil
	}

	if err != nil {
		return err
	}

	err = tx.Updates(map[string]interface{}{"grant_type": grantType, "deleted_at": nil}).Error
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) CheckUserPermissionToAccessNamespace(userId string, namespaceId string) (bool, bool, bool, error) {
	// read, write, admin (manage) の順に bool を返す
	var target NamespaceAccessPermission
	err := c.DB.Model(&NamespaceAccessPermission{}).Where("namespace_id = ?", namespaceId).Where("user_id = ?", userId).Where("deleted_at IS NULL").First(&target).Error
	if err != nil {
		return false, false, false, err
	}
	switch target.GrantType {
	case "r":
		return true, false, false, nil
	case "w":
		return false, true, false, nil
	case "rw":
		return true, true, false, nil
	case "admin":
		return true, true, true, nil
	default:
		return false, false, false, errors.New("invalid grant type")
	}
}

type _getCfgMeNamespaceResponse struct {
	NamespaceID    uuid.UUID `gorm:"column:namespace_id" json:"namespace_id"`
	OrganizationID uuid.UUID `gorm:"column:organization_id" json:"organization_id"`
	Name           string    `gorm:"column:name" json:"name"`
	GrantType      string    `gorm:"column:grant_type" json:"grant_type"`
}

type GetCfgMeNamespaceResponse []_getCfgMeNamespaceResponse

func (c *Controller) GetAvailableNamespaceList(userId uuid.UUID, offset, limit int) (GetCfgMeNamespaceResponse, error) {
	var res GetCfgMeNamespaceResponse
	err := c.DB.Model(&NamespaceAccessPermission{}).
		Select("namespace_access_permissions.namespace_id, namespace.organization_id, namespace.name, namespace_access_permissions.grant_type").
		Joins("LEFT JOIN namespace ON namespace.id = namespace_access_permissions.namespace_id AND namespace.deleted_at IS NULL").
		Where("namespace_access_permissions.user_id = ? AND namespace_access_permissions.deleted_at IS NULL", userId).
		Offset(offset).
		Limit(limit).
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Controller) CreateNamespace(userId uuid.UUID) (uuid.UUID, error) {
	var membership OrganizationMembership
	err := c.DB.Joins("JOIN organization ON organization.id = organization_membership.organization_id AND organization.deleted_at IS NULL").
		Where("organization_membership.user_id = ? AND organization_membership.role = ? AND organization_membership.deleted_at IS NULL AND organization.name = ?", userId, "owner", "Personal").
		Order("organization_membership.created_at").First(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		organization, createErr := c.CreateOrganization(userId, "Personal")
		if createErr != nil {
			return uuid.Nil, createErr
		}
		membership.OrganizationID = organization.ID
	} else if err != nil {
		return uuid.Nil, err
	}
	namespace, err := c.CreateNamespaceForOrganization(userId, membership.OrganizationID, "Default")
	if err != nil {
		return uuid.Nil, err
	}
	return namespace.ID, nil
}
