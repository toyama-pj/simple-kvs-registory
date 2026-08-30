package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
)

const passkeyCeremonyTTL = 5 * time.Minute

type webAuthnUser struct {
	user        lib.User
	credentials []webauthn.Credential
}

func (user *webAuthnUser) WebAuthnID() []byte {
	id := user.user.ID
	return id[:]
}

func (user *webAuthnUser) WebAuthnName() string { return user.user.Email }

func (user *webAuthnUser) WebAuthnDisplayName() string { return user.user.Name }

func (user *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return user.credentials }

type BeginPasskeyResponse struct {
	CeremonyID uuid.UUID `json:"ceremony_id"`
	Options    any       `json:"options" swaggertype:"object"`
}

type BeginPasskeyRegistrationRequest struct {
	Name string `json:"name" maxLength:"100"`
}

type PasskeyConfigResponse struct {
	Enabled bool `json:"enabled"`
}

// GetPasskeyConfigHandler exposes capability without security-sensitive RP details.
// @Summary Get passkey capability
// @Produce json
// @Success 200 {object} PasskeyConfigResponse
// @Router /auth/passkeys/config [get]
func (con *Controller) GetPasskeyConfigHandler(c fiber.Ctx) error {
	return c.JSON(PasskeyConfigResponse{Enabled: con.Config.PASSKEY_ENABLED})
}

// BeginPasskeyRegistrationHandler begins registration for an authenticated user.
// @Summary Begin passkey registration
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body BeginPasskeyRegistrationRequest false "Passkey label"
// @Success 200 {object} BeginPasskeyResponse
// @Router /auth/passkeys/register/begin [post]
func (con *Controller) BeginPasskeyRegistrationHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	if err := con.ensurePasskeysEnabled(c); err != nil {
		return err
	}
	request := BeginPasskeyRegistrationRequest{Name: "Passkey"}
	if len(c.Body()) != 0 {
		if err := c.Bind().Body(&request); err != nil {
			return invalidAuthRequest(c, "request body must be valid JSON")
		}
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 100 {
		return invalidAuthRequest(c, "passkey name must contain 1 to 100 characters")
	}
	user, err := con.loadWebAuthnUser(userID)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	web, err := con.newWebAuthn()
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	exclusions := make([]protocol.CredentialDescriptor, 0, len(user.credentials))
	for index := range user.credentials {
		exclusions = append(exclusions, user.credentials[index].Descriptor())
	}
	options, session, err := web.BeginRegistration(
		user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	ceremony, err := con.savePasskeyCeremony(&userID, "register", request.Name, session)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	return c.JSON(BeginPasskeyResponse{CeremonyID: ceremony.ID, Options: options})
}

// FinishPasskeyRegistrationHandler verifies and stores a new credential.
// @Summary Finish passkey registration
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Passkey-Ceremony-ID header string true "Ceremony ID"
// @Success 201 {object} lib.PasskeyCredential
// @Router /auth/passkeys/register/finish [post]
func (con *Controller) FinishPasskeyRegistrationHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	if err := con.ensurePasskeysEnabled(c); err != nil {
		return err
	}
	ceremony, session, err := con.consumePasskeyCeremony(c.Get("X-Passkey-Ceremony-ID"), "register", &userID)
	if err != nil {
		return invalidPasskeyCeremony(c)
	}
	user, err := con.loadWebAuthnUser(userID)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	web, err := con.newWebAuthn()
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	credential, err := web.FinishRegistration(user, session, webAuthnHTTPRequest(c.Body()))
	if err != nil {
		return invalidPasskeyResponse(c)
	}
	encoded, err := lib.NewJSONValue(credential)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	record := lib.PasskeyCredential{
		UserID:       userID,
		CredentialID: base64.RawURLEncoding.EncodeToString(credential.ID),
		Name:         ceremony.CredentialName,
		Credential:   encoded,
	}
	if err := con.DB.Create(&record).Error; err != nil {
		return con.passkeyInternalError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(record)
}

// BeginPasskeyLoginHandler begins a discoverable, usernameless passkey login.
// @Summary Begin passkey login
// @Produce json
// @Success 200 {object} BeginPasskeyResponse
// @Router /auth/passkeys/login/begin [post]
func (con *Controller) BeginPasskeyLoginHandler(c fiber.Ctx) error {
	if err := con.ensurePasskeysEnabled(c); err != nil {
		return err
	}
	web, err := con.newWebAuthn()
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	options, session, err := web.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	ceremony, err := con.savePasskeyCeremony(nil, "login", "", session)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	return c.JSON(BeginPasskeyResponse{CeremonyID: ceremony.ID, Options: options})
}

// FinishPasskeyLoginHandler verifies a discoverable credential and issues a session.
// @Summary Finish passkey login
// @Accept json
// @Produce json
// @Param X-Passkey-Ceremony-ID header string true "Ceremony ID"
// @Success 200 {object} lib.UserBearerTokenResponse
// @Router /auth/passkeys/login/finish [post]
func (con *Controller) FinishPasskeyLoginHandler(c fiber.Ctx) error {
	if err := con.ensurePasskeysEnabled(c); err != nil {
		return err
	}
	_, session, err := con.consumePasskeyCeremony(c.Get("X-Passkey-Ceremony-ID"), "login", nil)
	if err != nil {
		return invalidPasskeyCeremony(c)
	}
	web, err := con.newWebAuthn()
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	user, credential, err := web.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		userID, parseErr := uuid.FromBytes(userHandle)
		if parseErr != nil {
			return nil, errors.New("invalid passkey user handle")
		}
		credentialID := base64.RawURLEncoding.EncodeToString(rawID)
		var count int64
		if err := con.DB.Model(&lib.PasskeyCredential{}).Where("user_id = ? AND credential_id = ?", userID, credentialID).Count(&count).Error; err != nil || count != 1 {
			return nil, errors.New("passkey credential not found")
		}
		return con.loadWebAuthnUser(userID)
	}, session, webAuthnHTTPRequest(c.Body()))
	if err != nil {
		return invalidPasskeyResponse(c)
	}
	passkeyUser, ok := user.(*webAuthnUser)
	if !ok {
		return con.passkeyInternalError(c, errors.New("unexpected WebAuthn user type"))
	}
	credentialJSON, err := lib.NewJSONValue(credential)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	now := time.Now().UTC()
	credentialID := base64.RawURLEncoding.EncodeToString(credential.ID)
	if err := con.DB.Model(&lib.PasskeyCredential{}).
		Where("user_id = ? AND credential_id = ?", passkeyUser.user.ID, credentialID).
		Updates(map[string]any{"credential": credentialJSON, "last_used_at": now}).Error; err != nil {
		return con.passkeyInternalError(c, err)
	}
	token, err := con.ReturnLibController().CreateUserBearerToken(passkeyUser.user.ID)
	if err != nil {
		return con.passkeyInternalError(c, err)
	}
	con.setSessionCookie(c, token.Token, token.ExpiresAt)
	return c.JSON(token.Response())
}

// GetPasskeysHandler lists an authenticated user's passkeys without key material.
// @Summary List passkeys
// @Security BearerAuth
// @Produce json
// @Success 200 {array} lib.PasskeyCredential
// @Router /auth/passkeys [get]
func (con *Controller) GetPasskeysHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	var credentials []lib.PasskeyCredential
	if err := con.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&credentials).Error; err != nil {
		return con.passkeyInternalError(c, err)
	}
	return c.JSON(map[string]any{"data": credentials})
}

// DeletePasskeyHandler removes a passkey while preserving OTP account recovery.
// @Summary Delete a passkey
// @Security BearerAuth
// @Param passkey path string true "Passkey ID"
// @Success 204
// @Router /auth/passkeys/{passkey} [delete]
func (con *Controller) DeletePasskeyHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	passkeyID, err := uuid.Parse(c.Params("passkey"))
	if err != nil {
		return invalidAuthRequest(c, "passkey must be a UUID")
	}
	result := con.DB.Where("id = ? AND user_id = ?", passkeyID, userID).Delete(&lib.PasskeyCredential{})
	if result.Error != nil {
		return con.passkeyInternalError(c, result.Error)
	}
	if result.RowsAffected != 1 {
		return c.Status(fiber.StatusNotFound).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "Passkey not found", fiber.StatusNotFound, "passkey does not exist", c.Path()))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (con *Controller) newWebAuthn() (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          con.Config.PASSKEY_RP_ID,
		RPDisplayName: con.Config.PASSKEY_RP_DISPLAY_NAME,
		RPOrigins:     con.Config.PASSKEY_RP_ORIGINS,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
}

func (con *Controller) loadWebAuthnUser(userID uuid.UUID) (*webAuthnUser, error) {
	var user lib.User
	if err := con.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	var records []lib.PasskeyCredential
	if err := con.DB.Where("user_id = ?", userID).Find(&records).Error; err != nil {
		return nil, err
	}
	credentials := make([]webauthn.Credential, 0, len(records))
	for _, record := range records {
		var credential webauthn.Credential
		if err := json.Unmarshal(record.Credential, &credential); err != nil {
			return nil, fmt.Errorf("decode passkey credential %s: %w", record.ID, err)
		}
		credentials = append(credentials, credential)
	}
	return &webAuthnUser{user: user, credentials: credentials}, nil
}

func (con *Controller) savePasskeyCeremony(userID *uuid.UUID, flow, name string, session *webauthn.SessionData) (lib.PasskeyCeremony, error) {
	encoded, err := lib.NewJSONValue(session)
	if err != nil {
		return lib.PasskeyCeremony{}, err
	}
	now := time.Now().UTC()
	ceremony := lib.PasskeyCeremony{UserID: userID, Flow: flow, CredentialName: name, Session: encoded, CreatedAt: now, ExpiresAt: now.Add(passkeyCeremonyTTL)}
	if err := con.DB.Create(&ceremony).Error; err != nil {
		return lib.PasskeyCeremony{}, err
	}
	_ = con.DB.Where("expires_at < ?", now.Add(-time.Hour)).Delete(&lib.PasskeyCeremony{}).Error
	return ceremony, nil
}

func (con *Controller) consumePasskeyCeremony(id, flow string, userID *uuid.UUID) (lib.PasskeyCeremony, webauthn.SessionData, error) {
	ceremonyID, err := uuid.Parse(id)
	if err != nil {
		return lib.PasskeyCeremony{}, webauthn.SessionData{}, err
	}
	var ceremony lib.PasskeyCeremony
	err = con.DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ? AND flow = ? AND consumed_at IS NULL AND expires_at > ?", ceremonyID, flow, time.Now())
		if userID == nil {
			query = query.Where("user_id IS NULL")
		} else {
			query = query.Where("user_id = ?", *userID)
		}
		if err := query.First(&ceremony).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&lib.PasskeyCeremony{}).Where("id = ? AND consumed_at IS NULL", ceremony.ID).Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("passkey ceremony already consumed")
		}
		return nil
	})
	if err != nil {
		return lib.PasskeyCeremony{}, webauthn.SessionData{}, err
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(ceremony.Session, &session); err != nil {
		return lib.PasskeyCeremony{}, webauthn.SessionData{}, err
	}
	return ceremony, session, nil
}

func webAuthnHTTPRequest(body []byte) *http.Request {
	request, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func (con *Controller) ensurePasskeysEnabled(c fiber.Ctx) error {
	if !con.Config.PASSKEY_ENABLED {
		return c.Status(fiber.StatusNotFound).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "Passkeys disabled", fiber.StatusNotFound, "passkey authentication is not enabled", c.Path()))
	}
	return nil
}

func invalidPasskeyCeremony(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCErrorResponse(lib.ErrorAuthTokenError, "Invalid passkey ceremony", fiber.StatusUnauthorized, "passkey ceremony is invalid, expired, or already used", c.Path()))
}

func invalidPasskeyResponse(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCErrorResponse(lib.ErrorAuthTokenError, "Passkey verification failed", fiber.StatusUnauthorized, "passkey response could not be verified", c.Path()))
}

func (con *Controller) passkeyInternalError(c fiber.Ctx, err error) error {
	detail := "passkey operation failed"
	if con.Config.DEVELOPMENT {
		detail = err.Error()
	}
	return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Passkey operation failed", fiber.StatusInternalServerError, detail, c.Path()))
}
