package handlers

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
)

func (con *Controller) OrganizationHandlersSetup(router fiber.Router) {
	router.Use(con.AuthenticationMiddlewareHandler)
	router.Get("/", con.GetOrganizationsHandler)
	router.Post("/", con.PostOrganizationHandler)
	router.Get("/:organization/namespaces", con.GetOrganizationNamespacesHandler)
	router.Post("/:organization/namespaces", con.PostOrganizationNamespaceHandler)
}

func (con *Controller) NamespaceHandlersSetup(router fiber.Router) {
	router.Use(con.AuthenticationMiddlewareHandler)
	router.Get("/:namespace/devices", con.GetNamespaceDevicesHandler)
	router.Post("/:namespace/devices", con.PostNamespaceDeviceHandler)
	router.Get("/:namespace/measurements", con.GetNamespaceMeasurementsHandler)
}

func (con *Controller) DeviceHandlersSetup(router fiber.Router) {
	router.Use(con.AuthenticationMiddlewareHandler)
	router.Get("/:device", con.GetDeviceHandler)
	router.Patch("/:device", con.PatchDeviceHandler)
	router.Delete("/:device", con.DeleteDeviceHandler)
	router.Get("/:device/measurements", con.GetDeviceMeasurementsHandler)
}

type OrganizationWithRole struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateNamedResourceRequest struct {
	Name string `json:"name"`
}

type CreateDeviceRequest struct {
	Name    string `json:"name"`
	DevEUI  string `json:"dev_eui"`
	DevAddr string `json:"dev_addr"`
	AppSKey string `json:"app_s_key"`
	NwkSKey string `json:"nwk_s_key"`
}

type UpdateDeviceRequest struct {
	Name    *string `json:"name,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
	AppSKey *string `json:"app_s_key,omitempty"`
	NwkSKey *string `json:"nwk_s_key,omitempty"`
}

type MeasurementResponse struct {
	Data []lib.Measurement `json:"data"`
}

type OrganizationListResponse struct {
	Data []OrganizationWithRole `json:"data"`
}

type NamespaceListResponse struct {
	Data []lib.Namespace `json:"data"`
}

type DeviceListResponse struct {
	Data []lib.Device `json:"data"`
}

// GetOrganizationsHandler returns organizations the authenticated user belongs to.
// @Summary List organizations
// @Security BearerAuth
// @Produce json
// @Success 200 {object} OrganizationListResponse
// @Router /organizations [get]
func (con *Controller) GetOrganizationsHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	var result []OrganizationWithRole
	err := con.DB.Table("organization").
		Select("organization.id, organization.name, organization_membership.role, organization.created_at, organization.updated_at").
		Joins("JOIN organization_membership ON organization_membership.organization_id = organization.id AND organization_membership.deleted_at IS NULL").
		Where("organization_membership.user_id = ? AND organization.deleted_at IS NULL", userID).
		Order("organization.created_at, organization.id").
		Scan(&result).Error
	if err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.JSON(OrganizationListResponse{Data: result})
}

// PostOrganizationHandler creates an organization and makes the caller its owner.
// @Summary Create an organization
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateNamedResourceRequest true "Organization"
// @Success 201 {object} lib.Organization
// @Router /organizations [post]
func (con *Controller) PostOrganizationHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	var request CreateNamedResourceRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidResourceRequest(c, "request body must be valid JSON")
	}
	organization, err := con.ReturnLibController().CreateOrganization(userID, request.Name)
	if err != nil {
		if strings.Contains(err.Error(), "name must") {
			return invalidResourceRequest(c, err.Error())
		}
		return resourceDatabaseError(c, con.Config, err)
	}
	c.Location("/api/v1/organizations/" + organization.ID.String())
	return c.Status(fiber.StatusCreated).JSON(organization)
}

// GetOrganizationNamespacesHandler lists namespaces in an organization.
// @Summary List namespaces in an organization
// @Security BearerAuth
// @Produce json
// @Param organization path string true "Organization ID"
// @Success 200 {object} NamespaceListResponse
// @Router /organizations/{organization}/namespaces [get]
func (con *Controller) GetOrganizationNamespacesHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	organizationID, err := uuid.Parse(c.Params("organization"))
	if err != nil {
		return invalidResourceRequest(c, "organization must be a UUID")
	}
	if !con.isOrganizationMember(userID, organizationID) {
		return forbiddenResource(c)
	}
	var namespaces []lib.Namespace
	if err := con.DB.Where("organization_id = ?", organizationID).Order("created_at, id").Find(&namespaces).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.JSON(NamespaceListResponse{Data: namespaces})
}

// PostOrganizationNamespaceHandler creates a namespace in an organization.
// @Summary Create a namespace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization path string true "Organization ID"
// @Param request body CreateNamedResourceRequest true "Namespace"
// @Success 201 {object} lib.Namespace
// @Router /organizations/{organization}/namespaces [post]
func (con *Controller) PostOrganizationNamespaceHandler(c fiber.Ctx) error {
	userID, ok := requireUser(c)
	if !ok {
		return unauthorizedUserOnly(c)
	}
	organizationID, err := uuid.Parse(c.Params("organization"))
	if err != nil {
		return invalidResourceRequest(c, "organization must be a UUID")
	}
	var request CreateNamedResourceRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidResourceRequest(c, "request body must be valid JSON")
	}
	namespace, err := con.ReturnLibController().CreateNamespaceForOrganization(userID, organizationID, request.Name)
	if errors.Is(err, lib.ErrForbidden) {
		return forbiddenResource(c)
	}
	if err != nil {
		if strings.Contains(err.Error(), "name must") {
			return invalidResourceRequest(c, err.Error())
		}
		return resourceDatabaseError(c, con.Config, err)
	}
	c.Location("/api/v1/namespaces/" + namespace.ID.String())
	return c.Status(fiber.StatusCreated).JSON(namespace)
}

// GetNamespaceDevicesHandler lists devices in a namespace.
// @Summary List devices
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "Namespace ID"
// @Success 200 {object} DeviceListResponse
// @Router /namespaces/{namespace}/devices [get]
func (con *Controller) GetNamespaceDevicesHandler(c fiber.Ctx) error {
	userID, namespaceID, ok := con.authorizeNamespace(c, false)
	_ = userID
	if !ok {
		return nil
	}
	var devices []lib.Device
	if err := con.DB.Where("namespace_id = ?", namespaceID).Order("created_at, id").Find(&devices).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.JSON(DeviceListResponse{Data: devices})
}

// PostNamespaceDeviceHandler registers a LoRaWAN device and encrypted session keys.
// @Summary Register a device
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID"
// @Param request body CreateDeviceRequest true "Device and LoRaWAN 1.0.x session"
// @Success 201 {object} lib.Device
// @Router /namespaces/{namespace}/devices [post]
func (con *Controller) PostNamespaceDeviceHandler(c fiber.Ctx) error {
	_, namespaceID, ok := con.authorizeNamespace(c, true)
	if !ok {
		return nil
	}
	var request CreateDeviceRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidResourceRequest(c, "request body must be valid JSON")
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 100 {
		return invalidResourceRequest(c, "name must contain 1 to 100 characters")
	}
	devEUI, err := normalizeHexIdentifier(request.DevEUI, 8)
	if err != nil {
		return invalidResourceRequest(c, "dev_eui must be 16 hexadecimal characters")
	}
	devAddr, err := normalizeHexIdentifier(request.DevAddr, 4)
	if err != nil {
		return invalidResourceRequest(c, "dev_addr must be 8 hexadecimal characters")
	}
	appSKey, err := lib.EncryptSessionKey(con.Config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, request.AppSKey)
	if err != nil {
		return invalidResourceRequest(c, "app_s_key: "+err.Error())
	}
	nwkSKey, err := lib.EncryptSessionKey(con.Config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, request.NwkSKey)
	if err != nil {
		return invalidResourceRequest(c, "nwk_s_key: "+err.Error())
	}
	var duplicateCount int64
	if err := con.DB.Model(&lib.Device{}).Unscoped().Where("dev_eui = ? OR dev_addr = ?", devEUI, devAddr).Count(&duplicateCount).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	if duplicateCount != 0 {
		return c.Status(fiber.StatusConflict).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Device already exists", fiber.StatusConflict, "dev_eui and dev_addr must be unique", c.Path()))
	}
	device := lib.Device{
		NamespaceID:      namespaceID,
		Name:             request.Name,
		DevEUI:           devEUI,
		DevAddr:          devAddr,
		AppSKeyEncrypted: appSKey,
		NwkSKeyEncrypted: nwkSKey,
		Enabled:          true,
	}
	if err := con.DB.Create(&device).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	c.Location("/api/v1/devices/" + device.ID.String())
	return c.Status(fiber.StatusCreated).JSON(device)
}

// GetDeviceHandler returns a device without its session keys.
// @Summary Get a device
// @Security BearerAuth
// @Produce json
// @Param device path string true "Device ID"
// @Success 200 {object} lib.Device
// @Router /devices/{device} [get]
func (con *Controller) GetDeviceHandler(c fiber.Ctx) error {
	device, ok := con.authorizeDevice(c, false)
	if !ok {
		return nil
	}
	return c.JSON(device)
}

// PatchDeviceHandler updates display state or rotates both session keys.
// @Summary Update a device
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param device path string true "Device ID"
// @Param request body UpdateDeviceRequest true "Fields to update"
// @Success 200 {object} lib.Device
// @Router /devices/{device} [patch]
func (con *Controller) PatchDeviceHandler(c fiber.Ctx) error {
	device, ok := con.authorizeDevice(c, true)
	if !ok {
		return nil
	}
	var request UpdateDeviceRequest
	if err := c.Bind().Body(&request); err != nil {
		return invalidResourceRequest(c, "request body must be valid JSON")
	}
	updates := make(map[string]any)
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" || utf8.RuneCountInString(name) > 100 {
			return invalidResourceRequest(c, "name must contain 1 to 100 characters")
		}
		updates["name"] = name
	}
	if request.Enabled != nil {
		updates["enabled"] = *request.Enabled
	}
	if (request.AppSKey == nil) != (request.NwkSKey == nil) {
		return invalidResourceRequest(c, "app_s_key and nwk_s_key must be rotated together")
	}
	if request.AppSKey != nil {
		appSKey, err := lib.EncryptSessionKey(con.Config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, *request.AppSKey)
		if err != nil {
			return invalidResourceRequest(c, "app_s_key: "+err.Error())
		}
		nwkSKey, err := lib.EncryptSessionKey(con.Config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, *request.NwkSKey)
		if err != nil {
			return invalidResourceRequest(c, "nwk_s_key: "+err.Error())
		}
		updates["app_s_key_encrypted"] = appSKey
		updates["nwk_s_key_encrypted"] = nwkSKey
		updates["uplink_frame_counter"] = 0
		updates["has_uplink_frame"] = false
	}
	if len(updates) == 0 {
		return invalidResourceRequest(c, "at least one update field is required")
	}
	if err := con.DB.Model(&device).Updates(updates).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	if err := con.DB.First(&device, "id = ?", device.ID).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.JSON(device)
}

// DeleteDeviceHandler disables and soft-deletes a device.
// @Summary Delete a device
// @Security BearerAuth
// @Param device path string true "Device ID"
// @Success 204
// @Router /devices/{device} [delete]
func (con *Controller) DeleteDeviceHandler(c fiber.Ctx) error {
	device, ok := con.authorizeDevice(c, true)
	if !ok {
		return nil
	}
	if err := con.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&device).Update("enabled", false).Error; err != nil {
			return err
		}
		return tx.Delete(&device).Error
	}); err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetNamespaceMeasurementsHandler returns measurements for a namespace.
// @Summary Get namespace measurements
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "Namespace ID"
// @Param limit query int false "1 to 500"
// @Param before query int false "Unix seconds"
// @Param after query int false "Unix seconds"
// @Param device_id query string false "Device ID"
// @Param name query string false "Cayenne value name"
// @Param channel query int false "Cayenne channel"
// @Success 200 {object} MeasurementResponse
// @Router /namespaces/{namespace}/measurements [get]
func (con *Controller) GetNamespaceMeasurementsHandler(c fiber.Ctx) error {
	_, namespaceID, ok := con.authorizeNamespace(c, false)
	if !ok {
		return nil
	}
	return con.getMeasurements(c, namespaceID, uuid.Nil)
}

// GetDeviceMeasurementsHandler returns measurements for a device.
// @Summary Get device measurements
// @Security BearerAuth
// @Produce json
// @Param device path string true "Device ID"
// @Param limit query int false "1 to 500"
// @Param before query int false "Unix seconds"
// @Param after query int false "Unix seconds"
// @Param name query string false "Cayenne value name"
// @Param channel query int false "Cayenne channel"
// @Success 200 {object} MeasurementResponse
// @Router /devices/{device}/measurements [get]
func (con *Controller) GetDeviceMeasurementsHandler(c fiber.Ctx) error {
	device, ok := con.authorizeDevice(c, false)
	if !ok {
		return nil
	}
	return con.getMeasurements(c, device.NamespaceID, device.ID)
}

func (con *Controller) getMeasurements(c fiber.Ctx, namespaceID, deviceID uuid.UUID) error {
	limit, err := strconv.Atoi(c.Query("limit", "100"))
	if err != nil || limit < 1 || limit > 500 {
		return invalidResourceRequest(c, "limit must be between 1 and 500")
	}
	query := con.DB.Where("namespace_id = ?", namespaceID)
	if deviceID != uuid.Nil {
		query = query.Where("device_id = ?", deviceID)
	} else if value := c.Query("device_id"); value != "" {
		filterDevice, parseErr := uuid.Parse(value)
		if parseErr != nil {
			return invalidResourceRequest(c, "device_id must be a UUID")
		}
		query = query.Where("device_id = ?", filterDevice)
	}
	for param, operator := range map[string]string{"before": "<=", "after": ">="} {
		if value := c.Query(param); value != "" {
			unixSeconds, parseErr := strconv.ParseInt(value, 10, 64)
			if parseErr != nil {
				return invalidResourceRequest(c, param+" must be Unix time in seconds")
			}
			query = query.Where("received_at "+operator+" ?", time.Unix(unixSeconds, 0))
		}
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name = ?", name)
	}
	if value := c.Query("channel"); value != "" {
		channel, parseErr := strconv.ParseUint(value, 10, 8)
		if parseErr != nil {
			return invalidResourceRequest(c, "channel must be an integer between 0 and 255")
		}
		query = query.Where("channel = ?", channel)
	}
	var measurements []lib.Measurement
	if err := query.Order("received_at DESC").Order("id DESC").Limit(limit).Find(&measurements).Error; err != nil {
		return resourceDatabaseError(c, con.Config, err)
	}
	return c.JSON(MeasurementResponse{Data: measurements})
}

func (con *Controller) authorizeNamespace(c fiber.Ctx, manage bool) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := requireUser(c)
	if !ok {
		_ = unauthorizedUserOnly(c)
		return uuid.Nil, uuid.Nil, false
	}
	namespaceID, err := uuid.Parse(c.Params("namespace"))
	if err != nil {
		_ = invalidResourceRequest(c, "namespace must be a UUID")
		return uuid.Nil, uuid.Nil, false
	}
	canRead, _, canManage, err := con.ReturnLibController().CheckUserPermissionToAccessNamespace(userID.String(), namespaceID.String())
	if err != nil || (manage && !canManage) || (!manage && !canRead) {
		_ = forbiddenResource(c)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, namespaceID, true
}

func (con *Controller) authorizeDevice(c fiber.Ctx, manage bool) (lib.Device, bool) {
	userID, ok := requireUser(c)
	if !ok {
		_ = unauthorizedUserOnly(c)
		return lib.Device{}, false
	}
	deviceID, err := uuid.Parse(c.Params("device"))
	if err != nil {
		_ = invalidResourceRequest(c, "device must be a UUID")
		return lib.Device{}, false
	}
	var device lib.Device
	if err := con.DB.First(&device, "id = ?", deviceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Status(fiber.StatusNotFound).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "Device not found", fiber.StatusNotFound, "device does not exist", c.Path()))
		} else {
			_ = resourceDatabaseError(c, con.Config, err)
		}
		return lib.Device{}, false
	}
	canRead, _, canManage, err := con.ReturnLibController().CheckUserPermissionToAccessNamespace(userID.String(), device.NamespaceID.String())
	if err != nil || (manage && !canManage) || (!manage && !canRead) {
		_ = forbiddenResource(c)
		return lib.Device{}, false
	}
	return device, true
}

func (con *Controller) isOrganizationMember(userID, organizationID uuid.UUID) bool {
	var count int64
	return con.DB.Model(&lib.OrganizationMembership{}).
		Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", organizationID, userID).
		Count(&count).Error == nil && count == 1
}

func requireUser(c fiber.Ctx) (uuid.UUID, bool) {
	if _, writeToken := c.Locals("writeAccessTokenNamespaceId").(uuid.UUID); writeToken {
		return uuid.Nil, false
	}
	userID, ok := c.Locals("userId").(uuid.UUID)
	return userID, ok
}

func normalizeHexIdentifier(value string, bytes int) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != bytes {
		return "", errors.New("invalid hexadecimal identifier")
	}
	return value, nil
}

func unauthorizedUserOnly(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCUnauthorizedErrorResponse("a user bearer token is required", c.Path()))
}

func forbiddenResource(c fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonUnauthorized, "Forbidden", fiber.StatusForbidden, "you do not have permission to access this resource", c.Path()))
}

func invalidResourceRequest(c fiber.Ctx, detail string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid request", fiber.StatusUnprocessableEntity, detail, c.Path()))
}

func resourceDatabaseError(c fiber.Ctx, config lib.Config, err error) error {
	detail := "database operation failed"
	if config.DEVELOPMENT {
		detail = err.Error()
	}
	return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "Database error", fiber.StatusInternalServerError, detail, c.Path()))
}
