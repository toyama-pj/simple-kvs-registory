package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

// DataHandlersSetup
//
// @Tag.name			data
// @Tag.description	データ操作（読み書き）に関するAPI群
func (cont *Controller) DataHandlersSetup(router fiber.Router) {
	router.Use(cont.AuthenticationMiddlewareHandler)
	router.Get("/data/:namespace", cont.GetDataNamespaceHandler)
	router.Post("/data/:namespace", cont.PostDataNamespaceHandler)
}

type KeyValueRequestPayload struct {
	KeyValueWithTime []struct {
		Time      int `json:"time"`
		KeyValues []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
	}
}

type KeyValueResponsePayload struct {
	TimeValueWithKey []struct {
		Key        string `json:"key"`
		TimeValues []struct {
			Time  int    `json:"time"`
			Value string `json:"value"`
		} `json:"values"`
	}
}

func (payload *KeyValueResponsePayload) fromQueryResult(data []lib.Data) error {
	keyMap := make(map[string][]struct {
		Time  int    `json:"time"`
		Value string `json:"value"`
	})

	for _, d := range data {
		keyMap[d.Key] = append(keyMap[d.Key], struct {
			Time  int    `json:"time"`
			Value string `json:"value"`
		}{
			Time:  int(d.Time.Unix()),
			Value: d.Value,
		})
	}

	payload.TimeValueWithKey = make([]struct {
		Key        string `json:"key"`
		TimeValues []struct {
			Time  int    `json:"time"`
			Value string `json:"value"`
		} `json:"values"`
	}, 0, len(keyMap))

	for k, v := range keyMap {
		payload.TimeValueWithKey = append(payload.TimeValueWithKey, struct {
			Key        string `json:"key"`
			TimeValues []struct {
				Time  int    `json:"time"`
				Value string `json:"value"`
			} `json:"values"`
		}{
			Key:        k,
			TimeValues: v,
		})
	}

	return nil
}

// GetDataNamespaceHandler
// @Summary	ネームスペースのデータを取得
// @Description	指定したネームスペースから条件に合致するキー・バリューデータを取得する
// @Security BearerAuth
// @Produce	json
// @Param	namespace	path	string	true	"ネームスペースID (UUID)"
// @Param	beforeAt	query	int		false	"指定したUNIX時間以前のデータを取得"
// @Param	afterAt		query	int		false	"指定したUNIX時間以後のデータを取得"
// @Param	offset		query	int		false	"取得データのオフセット"
// @Param	limit		query	int		false	"取得データの最大件数（最大50件）"
// @Param	key			query	string	false	"特定のキー名で絞り込み"
// @Param	order		query	string	false	"時間の並び順 (ASC または DESC、デフォルトはDESC)"
// @Success	200		{object}	KeyValueResponsePayload		"取得されたデータ"
// @Failure	400		{object}	lib.RFCErrorResponse
// @Failure	401		{object}	lib.RFCErrorResponse
// @Failure 403		{object}	lib.RFCErrorResponse
// @Failure	500		{object}	lib.RFCErrorResponse
// @Router	/data/{namespace} [get]
func (con *Controller) GetDataNamespaceHandler(c fiber.Ctx) error {
	namespace, err := uuid.Parse(c.Params("namespace"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotUUID,
				"failed to parse: namespace",
				fiber.StatusBadRequest,
				"namespace is expected to be a valid UUID",
				c.Path(),
			),
		)
	}

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

	conn := con.ReturnLibController()
	canRead, _, _, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), namespace.String())
	if err != nil || !canRead {
		return c.Status(fiber.StatusForbidden).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorCommonUnauthorized,
				"Forbidden",
				fiber.StatusForbidden,
				"You don't have permission to read this namespace",
				c.Path(),
			),
		)
	}

	result := new(KeyValueResponsePayload)

	beforeInt, err := strconv.ParseInt(c.Get("beforeAt", "-1"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotInt,
				"failed to parse query param: beforeAt",
				fiber.StatusBadRequest,
				"beforeAt is expected for UNIX Time in integer",
				c.Path(),
			),
		)
	}

	afterInt, err := strconv.ParseInt(c.Get("afterAt", "-1"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotInt,
				"failed to parse query param: afterAt",
				fiber.StatusBadRequest,
				"afterAt is expected for UNIX Time in integer",
				c.Path(),
			),
		)
	}
	before := time.Unix(beforeInt, 0)
	after := time.Unix(afterInt, 0)
	offset, err := strconv.ParseInt(c.Get("offset", "0"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotInt,
				"failed to parse query param: offset",
				fiber.StatusBadRequest,
				"offset is expected for integer",
				c.Path(),
			),
		)
	}
	limit, err := strconv.ParseInt(c.Get("limit", "0"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotInt,
				"failed to parse query param: limit",
				fiber.StatusBadRequest,
				"limit is expected for integer",
				c.Path(),
			),
		)
	}
	key := c.Get("key", "")
	order := strings.ToUpper(c.Get("order", "DESC"))
	if order != "ASC" && order != "DESC" {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorRequestValueIsNotInExpectedValues,
				"invalid query param: order",
				fiber.StatusBadRequest,
				"order is expected to be either ASC or DESC",
				c.Path(),
			),
		)
	}

	filter := lib.Filter{
		Before:    before,
		After:     after,
		Namespace: namespace,
		Key:       key,
		Limit:     int(limit),
		Offset:    int(offset),
		TimeOrder: order,
	}

	rawRes, err := conn.ReadWithFilter(filter)
	if err != nil {
		if con.Config.DEVELOPMENT == true {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorDatabaseError,
					"failed to fetch data",
					fiber.StatusInternalServerError,
					err.Error(),
					c.Path(),
				),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				"failed to fetch data",
				fiber.StatusInternalServerError,
				"failed to fetch data",
				c.Path(),
			),
		)
	}

	err = result.fromQueryResult(rawRes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInternalServerError,
				"failed to fetch data",
				fiber.StatusInternalServerError,
				err.Error(),
				c.Path(),
			),
		)
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

// PostDataNamespaceHandler
// @Summary	ネームスペースへデータを書き込み
// @Description	指定したネームスペースへキー・バリューデータを一括で書き込む
// @Security BearerAuth
// @Accept	json
// @Produce	json
// @Param	namespace	path	string					true	"ネームスペースID (UUID)"
// @Param	request		body	KeyValueRequestPayload	true	"書き込むデータ"
// @Success	200		{object}	map[string]interface{}	"成功（空のJSON `{}` を返却）"
// @Failure	400		{object}	lib.RFCErrorResponse
// @Failure	401		{object}	lib.RFCErrorResponse
// @Failure 403		{object}	lib.RFCErrorResponse
// @Failure	500		{object}	lib.RFCErrorResponse
// @Router	/data/{namespace} [post]
func (con *Controller) PostDataNamespaceHandler(c fiber.Ctx) error {
	payload := new(KeyValueRequestPayload)
	if err := c.Bind().All(payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorInvalidRequest,
				"invalid payload format",
				fiber.StatusBadRequest,
				"payload needs JSON format, and needs time, key and values.",
				c.Path(),
			),
		)
	}
	conn := con.ReturnLibController()

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
	_, canWrite, _, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), c.Params("namespace"))
	if err != nil || !canWrite {
		return c.Status(fiber.StatusForbidden).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorCommonUnauthorized,
				"Forbidden",
				fiber.StatusForbidden,
				"You don't have permission to write to this namespace",
				c.Path(),
			),
		)
	}

	for _, p := range payload.KeyValueWithTime {
		for _, pp := range p.KeyValues {
			t := time.Unix(int64(p.Time), 0)
			data := lib.Data{
				Time:      t,
				Namespace: c.Params("namespace"),
				Key:       pp.Key,
				Value:     pp.Value,
			}
			if err := conn.Write(data); err != nil {
				if con.Config.DEVELOPMENT == true {
					return c.Status(fiber.StatusInternalServerError).JSON(
						lib.NewRFCErrorResponse(
							lib.ErrorDatabaseError,
							fmt.Sprintf("failed to write data - %s: %s", pp.Key, pp.Value),
							fiber.StatusInternalServerError,
							err.Error(),
							c.Path(),
						),
					)
				}

				return c.Status(fiber.StatusInternalServerError).JSON(
					lib.NewRFCErrorResponse(
						lib.ErrorInternalServerError,
						fmt.Sprintf("failed to write data"),
						fiber.StatusInternalServerError,
						"failed to write data",
						c.Path(),
					),
				)
			}
		}
	}
	return c.Status(fiber.StatusOK).JSON("{}")
}
