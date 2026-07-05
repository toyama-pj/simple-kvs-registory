package handlers

import (
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
	router.Post("/name", NotImplementedMiddlewareHandler)
	router.Post("/create", NotImplementedMiddlewareHandler)
	router.Get("/namespace", NotImplementedMiddlewareHandler)
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
					fiber.StatusInternalServerError,
					"err/me/database_error_get_user",
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				fiber.StatusInternalServerError,
				"err/me/internal_error",
				"Internal Server Error has occurred. Please retry later.",
				c.Path(),
			),
		)
	}

	return c.Status(fiber.StatusOK).JSON(user)
}
