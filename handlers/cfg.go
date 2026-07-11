package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func (cont *Controller) CfgHandlersSetup(router fiber.Router) {
	router.Use(cont.AuthenticationMiddlewareHandler)
	router.Route("/me", cont.CfgMeHandlersSetup)
	router.Route("/:namespace", cont.CfgNamespaceHandlersSetup)
}

func (cont *Controller) CfgMeHandlersSetup(router fiber.Router) {
	router.Get("/", cont.GetCfgMeHandler)
	router.Post("/name", cont.PostCfgMeNameHandler)
	router.Post("/create", cont.PostCfgMeNamespaceCreateHandler)
	router.Get("/namespace", cont.GetCfgMeNamespaceHandler)
}

func (cont *Controller) CfgNamespaceHandlersSetup(router fiber.Router) {
	router.Post("/invite", NotImplementedMiddlewareHandler)
	router.Post("/disinvite", NotImplementedMiddlewareHandler)
	router.Post("/wtoken/create", NotImplementedMiddlewareHandler)
	router.Post("/wtoken/revoke", NotImplementedMiddlewareHandler)
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
					fiber.StatusInternalServerError,
					"Database Error",
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				fiber.StatusInternalServerError,
				"Internal Server Error",
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
				fiber.StatusBadRequest,
				"Invalid Request",
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
				fiber.StatusInternalServerError,
				"Database Error",
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
				fiber.StatusBadRequest,
				"Invalid Request",
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
					fiber.StatusInternalServerError,
					"Database Error",
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				fiber.StatusInternalServerError,
				"Internal Server Error",
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
					fiber.StatusInternalServerError,
					"Database Error",
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				fiber.StatusInternalServerError,
				"Internal Server Error",
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
