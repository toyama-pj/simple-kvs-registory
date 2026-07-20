package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
)

func (cont *Controller) CfgHandlersSetup(router fiber.Router) {
	router.Use(cont.AuthenticationMiddlewareHandler)
	router.Route("/me", cont.CfgMeHandlersSetup)
	router.Route("/:namespace", cont.CfgNamespaceHandlersSetup)
}

func (cont *Controller) CfgMeHandlersSetup(router fiber.Router) {
	router.Get("/", cont.GetCfgMeHandler)
	router.Post("/name", cont.PostCfgMeNameHandler)
	router.Post("/namespace/create", cont.PostCfgMeNamespaceCreateHandler)
	router.Get("/namespace", cont.GetCfgMeNamespaceHandler)
}

func (cont *Controller) CfgNamespaceHandlersSetup(router fiber.Router) {
	router.Post("/invite", cont.PostCfgNamespaceInviteHandler)
	router.Post("/disinvite", cont.PostCfgNamespaceDisinviteHandler)
	router.Post("/token/create", cont.PostCfgNamespaceTokenCreateHandler)
	router.Post("/token/revoke", cont.PostCfgNamespaceTokenRevokeHandler)
}

// GetCfgMeHandler
// @Summary	自分自身の情報を表示
// @Description	自分自身の情報を表示する
// @Security BearerAuth
// @Accept	json
// @Produce	json
// @Success	200	{object}	lib.User	"ユーザの詳細情報"
// @Failure 401 {object}	lib.RFCErrorResponse
// @Failure 403 {object}	lib.RFCErrorResponse
// @Failure	500	{object}	lib.RFCErrorResponse
// @Router	/cfg/me [get]
func (con *Controller) GetCfgMeHandler(c fiber.Ctx) error {
	cont := con.ReturnLibController()
	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"unauthorized",
				c.Path(),
			),
		)
	}

	user, err := cont.GetUserById(userID)
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorDatabaseError,
					"Database Error",
					fiber.StatusInternalServerError,
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				"Internal Server Error",
				fiber.StatusInternalServerError,
				"Internal Server Error has occurred. Please retry later.",
				c.Path(),
			),
		)
	}

	return c.Status(fiber.StatusOK).JSON(user)
}

type PostCfgMeNameRequestBody struct {
	Name string `json:"name" validate:"required"`
}

// PostCfgMeNameHandler
// @Summary	自分のニックネームを変更する
// @Description	自分のニックネームを変更する
// @Security BearerAuth
// @Accept	json
// @Produce	json
// @Success	204	{object}	nil	"成功（返却ボディなし）"
// @Failure 401	{object}	lib.RFCErrorResponse
// @Failure 403 {object}	lib.RFCErrorResponse
// @Failure	500	{object}	lib.RFCErrorResponse
// @Router	/cfg/me [get]
func (con *Controller) PostCfgMeNameHandler(c fiber.Ctx) error {
	cont := lib.NewController(con.DB, con.Config)
	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"unauthorized",
				c.Path(),
			),
		)
	}
	var body PostCfgMeNameRequestBody
	if err := c.Bind().All(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				"Invalid Request",
				fiber.StatusBadRequest,
				err.Error(),
				c.Path(),
			),
		)
	}
	err := cont.ChangeUserNameById(userID, body.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorDatabaseError,
				"Database Error",
				fiber.StatusInternalServerError,
				err.Error(),
				c.Path(),
			),
		)
	}
	return c.Status(fiber.StatusNoContent).JSON("{}")
}

// GetCfgMeNamespaceHandler
// @Summary 自分のアクセスできる名前空間を取得する
// @Description	自分のアクセスできる名前空間を取得する
// @Security BearerAuth
// @Produce	json
// @Param	offset	query	int	false	"返却オフセット"
// @Success	200	{object}	lib.GetCfgMeNamespaceResponse	"ネームスペース一覧"
// @Failure 401	{object}	lib.RFCErrorResponse
// @Failure 403 {object}	lib.RFCErrorResponse
// @Failure	500	{object}	lib.RFCErrorResponse
// @Router	/cfg/me/namespace [get]
func (con *Controller) GetCfgMeNamespaceHandler(c fiber.Ctx) error {
	offset, err := strconv.Atoi(c.Get("offset", "0"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				"Invalid Request",
				fiber.StatusBadRequest,
				"Invalid offset parameter",
				c.Path(),
			),
		)
	}

	cont := con.ReturnLibController()
	userId := c.Locals("userId")
	userIdVal, ok := userId.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"unauthorized",
				c.Path(),
			),
		)
	}
	namespaceIds, err := cont.GetAvailableNamespaceList(userIdVal, offset)
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorDatabaseError,
					"Database Error",
					fiber.StatusInternalServerError,
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				"Internal Server Error",
				fiber.StatusInternalServerError,
				"Internal Server Error has occurred. Please retry later.",
				c.Path(),
			),
		)
	}
	return c.Status(fiber.StatusOK).JSON(
		namespaceIds,
	)
}

// PostCfgMeNamespaceCreateHandler
// @Summary KVS名前空間を作成する
// @Description	KVS名前空間を作成する
// @Security BearerAuth
// @Produce	json
// @Success	201	{object}	nil	"成功（返却ボディなし）"
// @Failure 401	{object}	lib.RFCErrorResponse
// @Failure 403 {object}	lib.RFCErrorResponse
// @Failure	500	{object}	lib.RFCErrorResponse
// @Router	/cfg/me/namespace/create [post]
func (con *Controller) PostCfgMeNamespaceCreateHandler(c fiber.Ctx) error {
	cont := con.ReturnLibController()
	userId := c.Locals("userId")
	userIdVal, ok := userId.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"unauthorized",
				c.Path(),
			),
		)
	}
	namespaceId, err := cont.CreateNamespace(userIdVal)
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorDatabaseError,
					"Database Error",
					fiber.StatusInternalServerError,
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				"Internal Server Error",
				fiber.StatusInternalServerError,
				"Internal Server Error has occurred. Please retry later.",
				c.Path(),
			),
		)
	}

	return c.Status(fiber.StatusCreated).JSON(
		map[string]interface{}{
			"namespace_id": namespaceId,
		},
	)
}

type PostCfgNamespaceInviteRequestBody struct {
	Email     string `json:"email" validate:"required,email"`
	GrantType string `json:"grant_type" validate:"required,oneof=r w rw admin"`
}

// PostCfgNamespaceInviteHandler
// @Summary ネームスペースへユーザーを招待する
// @Description ネームスペースに指定したメールアドレスのユーザーを招待し、権限を付与する
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "ネームスペースID (UUID)"
// @Param request body PostCfgNamespaceInviteRequestBody true "招待するユーザーのメールアドレスと権限"
// @Success 204 {object} nil "成功（返却ボディなし）"
// @Failure 400 {object} lib.RFCErrorResponse
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 403 {object} lib.RFCErrorResponse
// @Failure 500 {object} lib.RFCErrorResponse
// @Router /cfg/{namespace}/invite [post]
func (con *Controller) PostCfgNamespaceInviteHandler(c fiber.Ctx) error {
	namespace := c.Params("namespace")
	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCUnauthorizedErrorResponse("unauthorized", c.Path()))
	}

	var req PostCfgNamespaceInviteRequestBody
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid Request", fiber.StatusBadRequest, "Invalid request body", c.Path()))
	}

	conn := con.ReturnLibController()
	
	targetUser, err := conn.GetUserByMailAddress(req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "User Not Found", fiber.StatusBadRequest, "Target user not found", c.Path()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, "Failed to lookup user", c.Path()))
	}

	err = conn.PermitUserToAccessNamespace(userID.String(), targetUser.ID.String(), namespace, req.GrantType)
	if err != nil {
		if err.Error() == "doAsUser is not administrator" {
			return c.Status(fiber.StatusForbidden).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonUnauthorized, "Forbidden", fiber.StatusForbidden, "You must be an administrator to invite users", c.Path()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, err.Error(), c.Path()))
	}

	return c.Status(fiber.StatusNoContent).JSON("{}")
}

type PostCfgNamespaceDisinviteRequestBody struct {
	Email string `json:"email" validate:"required,email"`
}

// PostCfgNamespaceDisinviteHandler
// @Summary ネームスペースからユーザーを削除する
// @Description ネームスペースに指定したメールアドレスのユーザーの権限を剥奪する
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "ネームスペースID (UUID)"
// @Param request body PostCfgNamespaceDisinviteRequestBody true "削除するユーザーのメールアドレス"
// @Success 204 {object} nil "成功（返却ボディなし）"
// @Failure 400 {object} lib.RFCErrorResponse
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 403 {object} lib.RFCErrorResponse
// @Failure 500 {object} lib.RFCErrorResponse
// @Router /cfg/{namespace}/disinvite [post]
func (con *Controller) PostCfgNamespaceDisinviteHandler(c fiber.Ctx) error {
	namespace := c.Params("namespace")
	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCUnauthorizedErrorResponse("unauthorized", c.Path()))
	}

	var req PostCfgNamespaceDisinviteRequestBody
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid Request", fiber.StatusBadRequest, "Invalid request body", c.Path()))
	}

	conn := con.ReturnLibController()
	
	_, _, canManage, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), namespace)
	if err != nil || !canManage {
		return c.Status(fiber.StatusForbidden).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonUnauthorized, "Forbidden", fiber.StatusForbidden, "You must be an administrator to disinvite users", c.Path()))
	}

	targetUser, err := conn.GetUserByMailAddress(req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "User Not Found", fiber.StatusBadRequest, "Target user not found", c.Path()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, "Failed to lookup user", c.Path()))
	}

	if err := con.DB.Where("namespace_id = ? AND user_id = ?", namespace, targetUser.ID).Delete(&lib.NamespaceAccessPermission{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, "Failed to disinvite user", c.Path()))
	}

	return c.Status(fiber.StatusNoContent).JSON("{}")
}

// PostCfgNamespaceTokenCreateHandler
// @Summary ネームスペースのWriteAccessTokenを発行する
// @Description IoT機器などがデータ送信に使うためのWrite専用Bearer Tokenを発行する
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "ネームスペースID (UUID)"
// @Success 201 {object} lib.WriteAccessToken "作成されたトークン"
// @Failure 400 {object} lib.RFCErrorResponse
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 403 {object} lib.RFCErrorResponse
// @Failure 500 {object} lib.RFCErrorResponse
// @Router /cfg/{namespace}/token/create [post]
func (con *Controller) PostCfgNamespaceTokenCreateHandler(c fiber.Ctx) error {
	namespace := c.Params("namespace")
	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCUnauthorizedErrorResponse("unauthorized", c.Path()))
	}
	
	conn := con.ReturnLibController()
	_, _, canManage, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), namespace)
	if err != nil || !canManage {
		return c.Status(fiber.StatusForbidden).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorCommonUnauthorized,
				"Forbidden",
				fiber.StatusForbidden,
				"You don't have permission to manage this namespace",
				c.Path(),
			),
		)
	}

	nsUUID, err := uuid.Parse(namespace)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid UUID", fiber.StatusBadRequest, "Invalid namespace UUID", c.Path()))
	}

	token := lib.WriteAccessToken{
		NameSpaceID: nsUUID,
		Token: uuid.New(),
		CreatedAt: time.Now(),
		CreatedByUserID: userID,
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(10, 0, 0), // Valid for 10 years
	}
	if err := con.DB.Create(&token).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, "Failed to create token", c.Path()))
	}
	return c.Status(fiber.StatusCreated).JSON(token)
}

// PostCfgNamespaceTokenRevokeHandler
// @Summary ネームスペースのWriteAccessTokenを無効化する
// @Description IoT機器などがデータ送信に使うためのWrite専用Bearer Tokenを削除する
// @Security BearerAuth
// @Produce json
// @Param namespace path string true "ネームスペースID (UUID)"
// @Param token query string true "無効化するトークン(UUID)"
// @Success 204 {object} nil "成功（返却ボディなし）"
// @Failure 400 {object} lib.RFCErrorResponse
// @Failure 401 {object} lib.RFCErrorResponse
// @Failure 403 {object} lib.RFCErrorResponse
// @Failure 500 {object} lib.RFCErrorResponse
// @Router /cfg/{namespace}/token/revoke [post]
func (con *Controller) PostCfgNamespaceTokenRevokeHandler(c fiber.Ctx) error {
	namespace := c.Params("namespace")
	tokenStr := c.Query("token")

	userIdVal := c.Locals("userId")
	userID, ok := userIdVal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(lib.NewRFCUnauthorizedErrorResponse("unauthorized", c.Path()))
	}
	
	conn := con.ReturnLibController()
	_, _, canManage, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), namespace)
	if err != nil || !canManage {
		return c.Status(fiber.StatusForbidden).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorCommonUnauthorized,
				"Forbidden",
				fiber.StatusForbidden,
				"You don't have permission to manage this namespace",
				c.Path(),
			),
		)
	}

	if err := con.DB.Where("namespace_id = ? AND token = ?", namespace, tokenStr).Delete(&lib.WriteAccessToken{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "DB Error", fiber.StatusInternalServerError, "Failed to revoke token", c.Path()))
	}
	return c.Status(fiber.StatusNoContent).JSON("{}")
}
