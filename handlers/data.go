package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	router.Get("/:namespace", cont.GetDataNamespaceHandler)
	router.Post("/:namespace", cont.PostDataNamespaceHandler)
}

type KeyValueRequestPayload struct {
	KeyValueWithTime []KeyValuesAtTime `json:"keyValueWithTime" validate:"required,min=1,max=1000"`
}

type KeyValuesAtTime struct {
	Time      *int64     `json:"time" validate:"required" format:"int64" description:"Unix time in seconds"`
	KeyValues []KeyValue `json:"keyValues" validate:"required,min=1,max=1000"`
}

type KeyValue struct {
	Key   string  `json:"key" validate:"required,min=1,max=128" minLength:"1" maxLength:"128"`
	Value *string `json:"value" validate:"required" maxLength:"65536"`
}

const (
	maxWriteItems = 1000
	maxKeyLength  = 128
	maxValueBytes = 64 * 1024
)

type KeyValueResponsePayload struct {
	TimeValueWithKey []struct {
		Key        string `json:"key"`
		TimeValues []struct {
			Time  int64  `json:"time" format:"int64"`
			Value string `json:"value"`
		} `json:"values"`
	}
	NextCursor string `json:"next_cursor,omitempty"`
}

type dataCursor struct {
	Time time.Time `json:"time"`
	ID   uuid.UUID `json:"id"`
}

func (payload *KeyValueResponsePayload) fromQueryResult(data []lib.Data) error {
	keyMap := make(map[string][]struct {
		Time  int64  `json:"time" format:"int64"`
		Value string `json:"value"`
	})

	for _, d := range data {
		keyMap[d.Key] = append(keyMap[d.Key], struct {
			Time  int64  `json:"time" format:"int64"`
			Value string `json:"value"`
		}{
			Time:  d.Time.Unix(),
			Value: d.Value,
		})
	}

	payload.TimeValueWithKey = make([]struct {
		Key        string `json:"key"`
		TimeValues []struct {
			Time  int64  `json:"time" format:"int64"`
			Value string `json:"value"`
		} `json:"values"`
	}, 0, len(keyMap))

	for k, v := range keyMap {
		payload.TimeValueWithKey = append(payload.TimeValueWithKey, struct {
			Key        string `json:"key"`
			TimeValues []struct {
				Time  int64  `json:"time" format:"int64"`
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
// @Param	beforeAt	query	int		false	"指定したUNIX秒以前のデータを取得"
// @Param	afterAt		query	int		false	"指定したUNIX秒以後のデータを取得"
// @Param	offset		query	int		false	"取得データのオフセット（0以上、互換用。継続取得にはcursorを推奨）"
// @Param	cursor		query	string	false	"前レスポンスのnext_cursor。offsetとは併用不可"
// @Param	limit		query	int		false	"取得データの最大件数（1〜50、デフォルト50）"
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

	_, hasWriteToken := c.Locals("writeAccessTokenNamespaceId").(uuid.UUID)
	if hasWriteToken {
		return c.Status(fiber.StatusForbidden).JSON(
			lib.NewRFCErrorResponse(
				lib.ErrorCommonUnauthorized,
				"Forbidden",
				fiber.StatusForbidden,
				"WriteAccessToken cannot be used for reading data",
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

	var before time.Time
	if value := c.Query("beforeAt"); value != "" {
		beforeInt, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
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
		before = time.Unix(beforeInt, 0)
	}

	var after time.Time
	if value := c.Query("afterAt"); value != "" {
		afterInt, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
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
		after = time.Unix(afterInt, 0)
	}
	offset, err := strconv.ParseInt(c.Query("offset", "0"), 10, 64)
	if err != nil || offset < 0 {
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
	var cursorTime time.Time
	var cursorID uuid.UUID
	if encoded := c.Query("cursor"); encoded != "" {
		if offset != 0 {
			return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid pagination", fiber.StatusBadRequest, "cursor and offset cannot be used together", c.Path()))
		}
		cursorTime, cursorID, err = decodeDataCursor(encoded)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid cursor", fiber.StatusBadRequest, "cursor is malformed", c.Path()))
		}
	}
	limit, err := strconv.ParseInt(c.Query("limit", "50"), 10, 64)
	if err != nil || limit < 1 || limit > 50 {
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
		Before:     before,
		After:      after,
		Namespace:  namespace,
		Key:        key,
		Limit:      int(limit),
		Offset:     int(offset),
		TimeOrder:  order,
		CursorTime: cursorTime,
		CursorID:   cursorID,
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
	if len(rawRes) == int(limit) {
		result.NextCursor = encodeDataCursor(rawRes[len(rawRes)-1])
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func encodeDataCursor(data lib.Data) string {
	encoded, _ := json.Marshal(dataCursor{Time: data.Time, ID: data.ID})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeDataCursor(encoded string) (time.Time, uuid.UUID, error) {
	value, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	var cursor dataCursor
	if err := json.Unmarshal(value, &cursor); err != nil || cursor.Time.IsZero() || cursor.ID == uuid.Nil {
		if err == nil {
			err = fmt.Errorf("missing cursor fields")
		}
		return time.Time{}, uuid.Nil, err
	}
	return cursor.Time, cursor.ID, nil
}

// PostDataNamespaceHandler
// @Summary	ネームスペースへデータを書き込み
// @Description	指定したネームスペースへキー・バリューデータを一括で書き込む
// @Security BearerAuth
// @Accept	json
// @Produce	json
// @Param	namespace	path	string					true	"ネームスペースID (UUID)"
// @Param	request		body	KeyValueRequestPayload	true	"書き込むデータ"
// @Success	204		{object}	nil	"全件を同一トランザクションで保存（返却ボディなし）"
// @Failure	400		{object}	lib.RFCErrorResponse
// @Failure	401		{object}	lib.RFCErrorResponse
// @Failure 403		{object}	lib.RFCErrorResponse
// @Failure 413       {object} lib.RFCErrorResponse
// @Failure 422       {object} lib.RFCErrorResponse
// @Failure	500		{object}	lib.RFCErrorResponse
// @Router	/data/{namespace} [post]
func (con *Controller) PostDataNamespaceHandler(c fiber.Ctx) error {
	namespace, err := uuid.Parse(c.Params("namespace"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(lib.NewRFCErrorResponse(lib.ErrorRequestValueIsNotUUID, "Invalid namespace", fiber.StatusBadRequest, "namespace is expected to be a valid UUID", c.Path()))
	}
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
	if detail := validateKeyValuePayload(payload); detail != "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, "Invalid data", fiber.StatusUnprocessableEntity, detail, c.Path()))
	}
	conn := con.ReturnLibController()

	writeTokenNs, hasWriteToken := c.Locals("writeAccessTokenNamespaceId").(uuid.UUID)
	if hasWriteToken {
		if writeTokenNs != namespace {
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
	} else {
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
		_, canWrite, _, err := conn.CheckUserPermissionToAccessNamespace(userID.String(), namespace.String())
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
	}

	data := make([]lib.Data, 0, maxWriteItems)
	for _, p := range payload.KeyValueWithTime {
		for _, pp := range p.KeyValues {
			data = append(data, lib.Data{
				Time:      time.Unix(*p.Time, 0),
				Namespace: namespace.String(),
				Key:       pp.Key,
				Value:     *pp.Value,
			})
		}
	}
	if err := conn.WriteBatch(data); err != nil {
		detail := "failed to write data"
		if con.Config.DEVELOPMENT {
			detail = err.Error()
		}
		return c.Status(fiber.StatusInternalServerError).JSON(lib.NewRFCErrorResponse(lib.ErrorDatabaseError, "Failed to write data", fiber.StatusInternalServerError, detail, c.Path()))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func validateKeyValuePayload(payload *KeyValueRequestPayload) string {
	if len(payload.KeyValueWithTime) == 0 {
		return "keyValueWithTime must contain at least one item"
	}
	total := 0
	for _, group := range payload.KeyValueWithTime {
		if group.Time == nil {
			return "time is required and is Unix time in seconds"
		}
		if len(group.KeyValues) == 0 {
			return "keyValues must contain at least one item"
		}
		for _, item := range group.KeyValues {
			total++
			if total > maxWriteItems {
				return "a batch may contain at most 1000 key-value items"
			}
			if utf8.RuneCountInString(item.Key) == 0 || utf8.RuneCountInString(item.Key) > maxKeyLength {
				return "key must contain between 1 and 128 characters"
			}
			if item.Value == nil {
				return "value is required (an empty string is allowed)"
			}
			if len(*item.Value) > maxValueBytes {
				return "value may contain at most 65536 bytes"
			}
		}
	}
	return ""
}
