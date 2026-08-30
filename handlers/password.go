package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
)

const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type PasswordLoginRequest struct {
	Email    string `json:"email" maxLength:"254"`
	Password string `json:"password" maxLength:"128"`
}

type PasswordUpdateRequest struct {
	CurrentPassword string `json:"current_password,omitempty" maxLength:"128"`
	NewPassword     string `json:"new_password" minLength:"12" maxLength:"128"`
}

type PasswordStatusResponse struct {
	Configured bool `json:"configured"`
}

// PostPasswordLoginHandler authenticates a user with a configured password.
// @Summary Log in with password
// @Accept json
// @Produce json
// @Param request body PasswordLoginRequest true "Email and password"
// @Success 200 {object} lib.UserBearerTokenResponse
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 422 {object} lib.RFCErrorResponse
// @Failure 429 {object} lib.RFCErrorResponse
// @Router /auth/password/login [post]
func (con *Controller) PostPasswordLoginHandler(c fiber.Ctx) error {
	var request PasswordLoginRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidAuthRequest(c, "request body must be valid JSON")
	}
	request.Email = strings.TrimSpace(request.Email)
	if !validEmail(request.Email) || request.Password == "" || len(request.Password) > lib.PasswordMaxLength*4 {
		return invalidAuthRequest(c, "a valid email and password are required")
	}

	var user lib.User
	hash := dummyPasswordHash
	lookupErr := con.DB.First(&user, "email = ?", request.Email).Error
	if lookupErr == nil && user.PasswordHash != "" {
		hash = user.PasswordHash
	}
	matched, verifyErr := lib.VerifyPassword(hash, request.Password)
	if verifyErr != nil && lookupErr == nil && user.PasswordHash != "" {
		return con.passwordInternalError(c, verifyErr)
	}
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return con.passwordInternalError(c, lookupErr)
	}
	if lookupErr != nil || user.PasswordHash == "" || !matched {
		return invalidPasswordCredentials(c)
	}

	token, err := con.ReturnLibController().CreateUserBearerToken(user.ID)
	if err != nil {
		return con.passwordInternalError(c, err)
	}
	con.setSessionCookie(c, token.Token, token.ExpiresAt)
	return c.JSON(token.Response())
}

// GetPasswordStatusHandler reports whether the current user has a password.
// @Summary Get password status
// @Security BearerAuth
// @Produce json
// @Success 200 {object} PasswordStatusResponse
// @Router /auth/password [get]
func (con *Controller) GetPasswordStatusHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	var user lib.User
	if err := con.DB.Select("id", "password_hash").First(&user, "id = ?", userID).Error; err != nil {
		return con.passwordInternalError(c, err)
	}
	return c.JSON(PasswordStatusResponse{Configured: user.PasswordHash != ""})
}

// PutPasswordHandler sets or changes the current user's password. Existing
// passwords require re-authentication; after a change all other sessions are
// revoked while the session making the change remains active.
// @Summary Set or change password
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body PasswordUpdateRequest true "Current and new password"
// @Success 204
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 422 {object} lib.RFCErrorResponse
// @Router /auth/password [put]
func (con *Controller) PutPasswordHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	currentTokenID, ok := c.Locals("userBearerTokenId").(int)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	var request PasswordUpdateRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidAuthRequest(c, "request body must be valid JSON")
	}
	if err := lib.ValidateNewPassword(request.NewPassword); err != nil {
		return invalidAuthRequest(c, err.Error())
	}

	var user lib.User
	if err := con.DB.First(&user, "id = ?", userID).Error; err != nil {
		return con.passwordInternalError(c, err)
	}
	if user.PasswordHash != "" {
		matched, err := lib.VerifyPassword(user.PasswordHash, request.CurrentPassword)
		if err != nil {
			return con.passwordInternalError(c, err)
		}
		if !matched {
			return invalidPasswordCredentials(c)
		}
		if same, err := lib.VerifyPassword(user.PasswordHash, request.NewPassword); err != nil {
			return con.passwordInternalError(c, err)
		} else if same {
			return invalidAuthRequest(c, "new password must be different from the current password")
		}
	}

	passwordHash, err := lib.HashPassword(request.NewPassword)
	if err != nil {
		return con.passwordInternalError(c, err)
	}
	if err := con.DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&lib.User{}).Where("id = ?", userID)
		if user.PasswordHash == "" {
			query = query.Where("password_hash = ? OR password_hash IS NULL", "")
		} else {
			query = query.Where("password_hash = ?", user.PasswordHash)
		}
		result := query.Update("password_hash", passwordHash)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrInvalidTransaction
		}
		return tx.Where("user_id = ? AND id <> ?", userID, currentTokenID).Delete(&lib.UserBearerToken{}).Error
	}); err != nil {
		return con.passwordInternalError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func invalidPasswordCredentials(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCErrorResponse(lib.ErrorAuthTokenError, "Authentication failed", fiber.StatusUnauthorized, "email or password is invalid", c.Path()))
}

func (con *Controller) passwordInternalError(c fiber.Ctx, err error) error {
	detail := "password operation failed"
	if con.Config.DEVELOPMENT {
		detail = err.Error()
	}
	return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Internal Server Error", fiber.StatusInternalServerError, detail, c.Path()))
}
