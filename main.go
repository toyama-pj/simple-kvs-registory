package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func main() {
	app := fiber.New()

	app.Use(
		func(c fiber.Ctx) {
			err := lib.AccessLogMiddlewareHandler(c)
			if err != nil {
				log.Fatal(err)
			}
		})

	app.Use(
		func(c fiber.Ctx) {
			err := lib.AuthenticationMiddlewareHandler(c)
			if err != nil {
				log.Fatal(err)
				c.Status(fiber.StatusUnauthorized).SendString("Authorization Failed.")
			}
		})

	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString(c.Path())
	})

	app.Post("/*", func(c fiber.Ctx) error {
		return c.SendString("Post Received: " + c.Path())
	})

	log.Fatal(app.Listen(":3000"))

}

// ダミーの保存関数 (ここにGORMの `db.Create(&log)` などを書く)
/*func saveToDatabase(logData AccessLog) {
	// 確認用にJSON化してコンソールに出力
	b, _ := json.MarshalIndent(logData, "", "  ")
	fmt.Println(string(b))
}*/
