package lib

import (
	"crypto/rand"
	"errors"
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
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id"`
	Name      string         `gorm:"type:varchar;column:name"`
	Email     string         `gorm:"type:varchar;column:email;uniqueIndex"`
	CreatedAt time.Time      `gorm:"type:timestamptz;column:created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;column:updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" swaggerignore:"true"`
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
	Token     string         `gorm:"type:varchar;column:token" json:"token"`
	CreatedAt time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	ExpiresAt time.Time      `gorm:"type:timestamptz;column:expires_at" json:"expires_at"`
	DeletedAt gorm.DeletedAt `json:"-" swaggerignore:"true"`
}

type WriteAccessToken struct {
	ID              int            `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	NameSpaceID     uuid.UUID      `gorm:"column:namespace_id" json:"namespace_id"`
	Token           uuid.UUID      `gorm:"column:token" json:"token"`
	CreatedAt       time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	CreatedByUserID uuid.UUID      `gorm:"column:created_by_user_id" json:"created_by_user_id"`
	CreatedBy       User           `gorm:"foreignKey:CreatedByUserID" json:"created_by"`
	UpdatedAt       time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	ExpiresAt       time.Time      `gorm:"type:timestamptz;column:expires_at" json:"expires_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

type NamespaceAccessPermission struct {
	ID          int       `gorm:"primaryKey;column:id;autoIncrement"`
	NamespaceID uuid.UUID `gorm:"primaryKey;type:uuid;column:namespace_id"`
	UserID      uuid.UUID `gorm:"primaryKey;type:uuid;column:user_id"`
	GrantType   string    `gorm:"column:grant_type"` // r, w, rw, admin
	CreatedAt   time.Time `gorm:"type:timestamptz;column:created_at"`
	DeletedAt   gorm.DeletedAt
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
		res, err := createRandomDigit(6)
		if err != nil {
			return "", err
		}
		u := UserOneTimeLogin{
			UserID:    userID,
			Token:     res,
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
	u := UserRegistration{
		Name:      name,
		Email:     email,
		Token:     res,
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
	var reg UserRegistration
	err := c.DB.Where("email = ? AND token = ?", email, code).
		Where("expires_at > ?", time.Now()).
		Order("created_at desc").
		First(&reg).Error
	
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Create user
	user := User{
		ID:    uuid.New(),
		Name:  reg.Name,
		Email: reg.Email,
	}

	err = c.DB.Create(&user).Error
	if err != nil {
		return nil, err
	}

	// Clean up used and old registration codes for this email
	c.DB.Where("email = ?", email).Delete(&UserRegistration{})

	return &user, nil
}

func (c *Controller) GetUserOneTimeLoginCode(email string, token string) (User, error) {
	var user User
	err := c.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return User{}, err
	}

	var onetime UserOneTimeLogin
	err = c.DB.Where("token = ?", token).Where("expires_at > ?", time.Now()).Where("user_id = ?", user.ID).First(&onetime).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrInvalidToken
	}
	if err != nil {
		return User{}, err
	}

	err = c.DB.Model(&onetime).Update("expires_at", time.Now()).Error
	if err != nil {
		return User{}, err
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
			Token:     token,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour * 24),
		}
		err = c.DB.Create(&u).Error
		if err != nil {
			return UserBearerToken{}, err
		}
		return u, nil
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
	err = c.DB.Model(&NamespaceAccessPermission{}).Where("namespace_id = ?", namespaceId).First(&doAs).Error
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
	tx := c.DB.Model(&NamespaceAccessPermission{}).Where("namespace_id = ?", namespaceId).First(&target)
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
		err = c.DB.Create(target).Error
		if err != nil {
			return err
		}
		return nil
	}

	if err != nil {
		return err
	}

	err = tx.Update("grant_type", grantType).Error
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
		return true, true, false, nil
	case "rw":
		return true, true, false, nil
	case "admin":
		return true, true, true, nil
	default:
		return false, false, false, errors.New("invalid grant type")
	}
}

type _getCfgMeNamespaceResponse struct {
	NamespaceID uuid.UUID `gorm:"column:namespace_id" json:"namespace_id"`
	GrantType   string    `gorm:"column:grant_type" json:"grant_type"`
}

type GetCfgMeNamespaceResponse []_getCfgMeNamespaceResponse

func (c *Controller) GetAvailableNamespaceList(userId uuid.UUID, offset int) (GetCfgMeNamespaceResponse, error) {
	var res GetCfgMeNamespaceResponse
	err := c.DB.Model(&NamespaceAccessPermission{}).
		Select("namespace_id, grant_type").
		Where("user_id = ?", userId).
		Offset(offset).
		Limit(10).
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Controller) CreateNamespace(userId uuid.UUID) (uuid.UUID, error) {
	namespaceId := uuid.New()
	err := c.DB.Create(&NamespaceAccessPermission{
		NamespaceID: namespaceId,
		UserID:      userId,
		GrantType:   "admin",
	}).Error
	if err != nil {
		return uuid.Nil, err
	}
	return namespaceId, nil
}
