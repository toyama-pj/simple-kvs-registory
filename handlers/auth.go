package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
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
	router.Post("/login/callback", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 10 * time.Minute,
	}), cont.PostLoginHandler)
	router.Post("/register", cont.PostRegisterHandler)
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
				"Invalid Request",
				fiber.StatusBadRequest,
				"Request is not valid",
				c.Path(),
			),
		)
	}

	cont := con.ReturnLibController()
	u, err := cont.GetUserByMailAddress(req.Email)
	if err == gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusNoContent).JSON("{}")
	}
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "Database Error", fiber.StatusInternalServerError, err.Error(), c.Path()),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Internal Server Error", fiber.StatusInternalServerError, "Internal Server Error has occurred. Please retry later.", c.Path()),
		)
	}

	code, err := cont.CreateUserOneTimeLoginCode(u.ID)
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "Database Error", fiber.StatusInternalServerError, err.Error(), c.Path()),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Internal Server Error", fiber.StatusInternalServerError, "Internal Server Error has occurred. Please retry later.", c.Path()),
		)
	}

	if con.Config.SMTP_USERNAME != "" {
		err = cont.SendOneTimeLoginCode(req.Email, code)
		if err != nil {
			fmt.Printf("Failed to send email to %s: %v\n", req.Email, err)
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Sending Failed", fiber.StatusInternalServerError, "Failed to send one-time login code.", c.Path()),
			)
		}
	} else if con.Config.DEVELOPMENT == true {
		fmt.Println("Because SMTP_USERNAME is empty, One Time Login Code is not sending.")
		fmt.Printf("Sent to %s, and Code is %s\n", req.Email, code)
	} else {
		fmt.Println("SMTP is not configured, cannot send email in PRODUCTION")
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Configuration Error", fiber.StatusInternalServerError, "SMTP is not configured.", c.Path()),
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
				"Invalid Request",
				fiber.StatusBadRequest,
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
				"Invalid Token",
				fiber.StatusUnauthorized,
				"Invalid token",
				c.Path(),
			),
		)
	} else if err != nil {
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

	token, err := cont.CreateUserBearerToken(u.ID)
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

	return c.Status(fiber.StatusOK).JSON(token)
}

type PostRegisterRequestBody struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

// PostRegisterHandler
// @Summary		ユーザーの新規登録
// @Description	名前とメールアドレスを使用して新しいユーザーを作成する
// @Accept		json
// @Produce		json
// @Param		request	body		PostRegisterRequestBody	true	"Name and Email"
// @Success		201		{object}	nil				"成功（返却ボディなし）"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure		500		{object}	lib.RFCErrorResponse
// @Router		/auth/register [post]
func (con *Controller) PostRegisterHandler(c fiber.Ctx) error {
	req := new(PostRegisterRequestBody)
	if err := c.Bind().All(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				"Invalid Request",
				fiber.StatusBadRequest,
				"Request is not valid",
				c.Path(),
			),
		)
	}

	cont := con.ReturnLibController()
	err := cont.CreateUser(req.Name, req.Email)
	if err != nil {
		if err.Error() == "user already exists" {
			return c.Status(fiber.StatusBadRequest).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorInvalidRequest,
					"User already exists",
					fiber.StatusBadRequest,
					"A user with this email already exists",
					c.Path(),
				),
			)
		}
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

	return c.Status(fiber.StatusCreated).JSON("{}")
}
