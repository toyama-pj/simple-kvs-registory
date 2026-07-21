package handlers

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

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
	router.Post("/login", cont.authIPLimiter(5), cont.authEmailLimiter(5), cont.PostLoginOneTimeCodeHandler)
	router.Post("/login/callback", cont.authIPLimiter(5), cont.authEmailLimiter(5), cont.PostLoginHandler)
	router.Post("/register", cont.authIPLimiter(3), cont.authEmailLimiter(3), cont.PostRegisterHandler)
	router.Post("/register/callback", cont.authIPLimiter(5), cont.authEmailLimiter(5), cont.PostRegisterCallbackHandler)
}

func (cont *Controller) authIPLimiter(max int) fiber.Handler {
	return limiter.New(limiter.Config{Max: max, Expiration: 10 * time.Minute, LimitReached: cont.TooManyRequestsHandler})
}

func (cont *Controller) authEmailLimiter(max int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   10 * time.Minute,
		LimitReached: cont.TooManyRequestsHandler,
		KeyGenerator: func(c fiber.Ctx) string {
			var request struct {
				Email string `json:"email"`
			}
			_ = c.Bind().Body(&request)
			return strings.ToLower(strings.TrimSpace(request.Email))
		},
	})
}

func (cont *Controller) TooManyRequestsHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Too Many Requests", fiber.StatusTooManyRequests, "Rate limit exceeded; retry after the time in the Retry-After header", c.Path()))
}

type PostLoginOneTimeCodeRequestBody struct {
	Email string `json:"email" validate:"required,email,max=254" maxLength:"254"`
}

// PostLoginOneTimeCodeHandler
// @Summary		ログインワンタイムパスワードを生成
// @Description	メールアドレスをキーとしてユーザを照合し、6桁・有効期限10分のログインコードを生成・送信する。同一ユーザーの以前のコードは無効になり、成功・未登録とも204を返す。IP・メールアドレス単位で10分に5回まで。
// @Accept		json
// @Produce		json
// @Param		request	body		PostLoginOneTimeCodeRequestBody	true	"Email"
// @Success		204		{object}	nil				"成功（返却ボディなし）"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure     422     {object} lib.RFCErrorResponse
// @Failure     429     {object} lib.RFCErrorResponse
// @Failure     503     {object} lib.RFCErrorResponse
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
	if !validEmail(req.Email) {
		return invalidAuthRequest(c, "email must be a valid address of at most 254 characters")
	}

	cont := con.ReturnLibController()
	u, err := cont.GetUserByMailAddress(req.Email)
	if err == gorm.ErrRecordNotFound {
		return c.SendStatus(fiber.StatusNoContent)
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
			return c.Status(fiber.StatusServiceUnavailable).JSON(
				lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Sending Failed", fiber.StatusServiceUnavailable, "Failed to send one-time login code.", c.Path()),
			)
		}
	} else if con.Config.DEVELOPMENT == true {
		fmt.Println("Because SMTP_USERNAME is empty, One Time Login Code is not sending.")
		fmt.Printf("Sent to %s, and Code is %s\n", req.Email, code)
	} else {
		fmt.Println("SMTP is not configured, cannot send email in PRODUCTION")
		return c.Status(fiber.StatusServiceUnavailable).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Configuration Error", fiber.StatusServiceUnavailable, "SMTP is not configured.", c.Path()),
		)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type PostLoginRequestBody struct {
	Email string `json:"email" validate:"required,email,max=254" maxLength:"254"`
	Code  string `json:"code" validate:"required,len=6"`
}

// PostLoginHandler
// @Summary	ログインワンタイムコードを検証
// @Description	メールアドレスと6桁のログインコードを検証し、Bearer Token を返却する。コードは一度だけ利用でき、有効期限は10分。IP・メールアドレス単位で10分に5回まで。
// @Accept		json
// @Produce		json
// @Param		request	body		PostLoginRequestBody	true	"Email and Code"
// @Success		200		{object}	lib.UserBearerTokenResponse		"Bearer Token"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure		401		{object}	lib.RFCErrorResponse
// @Failure     422     {object} lib.RFCErrorResponse
// @Failure     429     {object} lib.RFCErrorResponse
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
	if !validEmail(req.Email) || !validOneTimeCode(req.Code) {
		return invalidAuthRequest(c, "email and a 6-digit code are required")
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

	return c.Status(fiber.StatusOK).JSON(token.Response())
}

type PostRegisterRequestBody struct {
	Name  string `json:"name" validate:"required,max=100" maxLength:"100"`
	Email string `json:"email" validate:"required,email,max=254" maxLength:"254"`
}

// PostRegisterHandler
// @Summary		ユーザー登録コードを発行
// @Description	名前とメールアドレスを一時登録し、6桁・有効期限30分の登録コードを送信する。IP・メールアドレス単位で10分に3回まで。
// @Accept		json
// @Produce		json
// @Param		request	body		PostRegisterRequestBody	true	"Name and Email"
// @Success		204		{object}	nil				"成功（返却ボディなし）"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure     409     {object} lib.RFCErrorResponse
// @Failure     422     {object} lib.RFCErrorResponse
// @Failure		500		{object}	lib.RFCErrorResponse
// @Failure     429     {object} lib.RFCErrorResponse
// @Failure     503     {object} lib.RFCErrorResponse
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
	if strings.TrimSpace(req.Name) == "" || utf8.RuneCountInString(req.Name) > 100 || !validEmail(req.Email) {
		return invalidAuthRequest(c, "name must contain 1 to 100 characters and email must be valid")
	}

	cont := con.ReturnLibController()
	code, err := cont.CreateUserRegistrationCode(req.Name, req.Email)
	if err != nil {
		if err.Error() == "user already exists" {
			// 同様にセキュリティの観点からエラーとせず、既存ユーザにログインワンタイムパスワードを送るなどの分岐も検討可能ですが
			// 今回はシンプルに重複エラーを返します
			return c.Status(fiber.StatusConflict).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorInvalidRequest,
					"User already exists",
					fiber.StatusConflict,
					"A user with this email already exists",
					c.Path(),
				),
			)
		}
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
		err = cont.SendRegistrationCode(req.Email, code)
		if err != nil {
			fmt.Printf("Failed to send registration email to %s: %v\n", req.Email, err)
			return c.Status(fiber.StatusServiceUnavailable).JSON(
				lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Sending Failed", fiber.StatusServiceUnavailable, "Failed to send registration code.", c.Path()),
			)
		}
	} else if con.Config.DEVELOPMENT == true {
		fmt.Println("Because SMTP_USERNAME is empty, Registration Code is not sending.")
		fmt.Printf("Sent to %s, and Registration Code is %s\n", req.Email, code)
	} else {
		fmt.Println("SMTP is not configured, cannot send registration email in PRODUCTION")
		return c.Status(fiber.StatusServiceUnavailable).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Email Configuration Error", fiber.StatusServiceUnavailable, "SMTP is not configured.", c.Path()),
		)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

type PostRegisterCallbackRequestBody struct {
	Email string `json:"email" validate:"required,email,max=254" maxLength:"254"`
	Code  string `json:"code" validate:"required,len=6"`
}

// PostRegisterCallbackHandler
// @Summary		ユーザー登録ワンタイムコードを検証
// @Description	メールアドレスと登録ワンタイムパスコードを検証し、ユーザーを作成して Bearer Token を返却する
// @Accept		json
// @Produce		json
// @Param		request	body		PostRegisterCallbackRequestBody	true	"Email and Code"
// @Success		201		{object}	lib.UserBearerTokenResponse		"Bearer Token"
// @Failure		400		{object}	lib.RFCErrorResponse
// @Failure		401		{object}	lib.RFCErrorResponse
// @Failure     422     {object} lib.RFCErrorResponse
// @Failure     429     {object} lib.RFCErrorResponse
// @Router		/auth/register/callback [post]
func (con *Controller) PostRegisterCallbackHandler(c fiber.Ctx) error {
	req := new(PostRegisterCallbackRequestBody)
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
	if !validEmail(req.Email) || !validOneTimeCode(req.Code) {
		return invalidAuthRequest(c, "email and a 6-digit code are required")
	}

	cont := con.ReturnLibController()
	u, err := cont.VerifyUserRegistrationCode(req.Email, req.Code)
	if errors.Is(err, lib.ErrInvalidToken) {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorAuthTokenError,
				"Invalid Token",
				fiber.StatusUnauthorized,
				"Invalid token or expired",
				c.Path(),
			),
		)
	} else if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "Database Error", fiber.StatusInternalServerError, err.Error(), c.Path()),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(lib.ErrorInternalServerError, "Internal Server Error", fiber.StatusInternalServerError, "Internal Server Error has occurred. Please retry later.", c.Path()),
		)
	}

	token, err := cont.CreateUserBearerToken(u.ID)
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

	return c.Status(fiber.StatusCreated).JSON(token.Response())
}

func validEmail(value string) bool {
	if value == "" || len(value) > 254 {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

func validOneTimeCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func invalidAuthRequest(c fiber.Ctx, detail string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid Request", fiber.StatusUnprocessableEntity, detail, c.Path()))
}
