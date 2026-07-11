package handlers

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
)

type Controller struct {
	DB     *gorm.DB
	Config lib.Config
}

func NewController(db *gorm.DB, config lib.Config) *Controller {
	return &Controller{DB: db, Config: config}
}

func (cont *Controller) ReturnLibController() *lib.Controller {
	return lib.NewController(cont.DB, cont.Config)
}

// AuthHandlersSetup
//
// @Tag.name			auth
// @Tag.description	認証・ログインに関するAPI群
func (cont *Controller) AuthHandlersSetup(router fiber.Router) {
	router.Post("/login", cont.PostLoginOneTimeCodeHandler)
	router.Post("/login/callback", cont.PostLoginHandler)
}

type PostLoginOneTimeCodeRequestBody struct {
	Email string `json:"email" validate:"required,email"`
}

// PostLoginOneTimeCodeHandler
// @Summary		ログインワンタイムパスワードを生成
// @Description	メールアドレスをキーとしてユーザを照合し、ログインワンタイムパスワードを生成・送信
// @Accept		json
// @Produce		json
// @Param		request	body		PostLoginOneTimeCodeRequestBody	true	"Email"
// @Success		204		{object}	nil				"成功（返却ボディなし）"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Router		/auth/login [post]
func (con *Controller) PostLoginOneTimeCodeHandler(c fiber.Ctx) error {
	req := new(PostLoginOneTimeCodeRequestBody)
	if err := c.Bind().All(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				fiber.StatusBadRequest,
				"Invalid Request",
				"Request is not valid",
				c.Path(),
			),
		)
	}

	if con.Config.DEVELOPMENT == true {
		cont := con.ReturnLibController()
		u, err := cont.GetUserByMailAddress(req.Email)
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNoContent).JSON("{}")
		}
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
		code, err := cont.CreateUserOneTimeLoginCode(u.ID)
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
		fmt.Println("Because of Development mode, One Time Login Code is not sending.")
		fmt.Printf("Sent to %s, and Code is %s\n", req.Email, code)
	} else {
		return c.Status(fiber.StatusNotImplemented).JSON(
			lib.NewRFCNotImplementErrorResponse(c.Path()),
		)
	}
	return c.Status(fiber.StatusNoContent).JSON("{}")
}

type PostLoginRequestBody struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code" validate:"required,len=6"`
}

// PostLoginHandler
// @Summary	ログインワンタイムコードを検証
// @Description	メールアドレスとログインワンタイムパスコードを検証し、Bearer Token を返却
// @Accept		json
// @Produce		json
// @Param		request	body		PostLoginRequestBody	true	"Email and Code"
// @Success		200		{object}	lib.UserBearerToken		"Bearer Token"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure		401		{object}	lib.RFCErrorResponse
// @Router		/auth/login/callback [post]
func (con *Controller) PostLoginHandler(c fiber.Ctx) error {
	req := new(PostLoginRequestBody)
	if err := c.Bind().All(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				fiber.StatusBadRequest,
				"Invalid Request",
				"Request is not valid",
				c.Path(),
			),
		)
	}

	cont := con.ReturnLibController()
	u, err := cont.GetUserOneTimeLoginCode(req.Email, req.Code)
	if errors.Is(err, lib.ErrInvalidToken) {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorAuthTokenError,
				fiber.StatusUnauthorized,
				"Invalid Token",
				"Invalid token",
				c.Path(),
			),
		)
	} else if err != nil {
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

	token, err := cont.CreateUserBearerToken(u.ID)
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

	return c.Status(fiber.StatusOK).JSON(token)
}
