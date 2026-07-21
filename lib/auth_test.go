package lib_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	database "github.com/toyama-pj/simple-kvs-registory/lib/db"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testController(t *testing.T) *lib.Controller {
	t.Helper()
	db, err := gorm.Open(database.GetDatabaseDialector("duckdb", ":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := lib.MigrateSchema(db); err != nil {
			t.Fatalf("MigrateSchema run %d: %v", run+1, err)
		}
	}
	return lib.NewController(db, lib.Config{})
}

func TestNamespacePermissionChecksExactActorAndTarget(t *testing.T) {
	controller := testController(t)
	admin := lib.User{ID: uuid.New(), Name: "admin", Email: "admin@example.com"}
	outsider := lib.User{ID: uuid.New(), Name: "outsider", Email: "outsider@example.com"}
	target := lib.User{ID: uuid.New(), Name: "target", Email: "target@example.com"}
	for _, user := range []lib.User{admin, outsider, target} {
		if err := controller.DB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	namespaceID, err := controller.CreateNamespace(admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := controller.PermitUserToAccessNamespace(outsider.ID.String(), target.ID.String(), namespaceID.String(), "r"); err == nil {
		t.Fatal("non-admin was allowed to grant access")
	}
	if err := controller.PermitUserToAccessNamespace(admin.ID.String(), target.ID.String(), namespaceID.String(), "w"); err != nil {
		t.Fatal(err)
	}
	canRead, canWrite, canManage, err := controller.CheckUserPermissionToAccessNamespace(target.ID.String(), namespaceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if canRead || !canWrite || canManage {
		t.Fatalf("w permission returned read=%v write=%v manage=%v", canRead, canWrite, canManage)
	}
	if err := controller.PermitUserToAccessNamespace(admin.ID.String(), target.ID.String(), namespaceID.String(), "r"); err != nil {
		t.Fatal(err)
	}
	var grants int64
	if err := controller.DB.Model(&lib.NamespaceAccessPermission{}).Where("namespace_id = ? AND user_id = ?", namespaceID, target.ID).Count(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != 1 {
		t.Fatalf("grant rows = %d, want 1", grants)
	}
}

func TestLoginCodeIsSingleUseAndBearerTokenIsHashed(t *testing.T) {
	controller := testController(t)
	user := lib.User{ID: uuid.New(), Name: "user", Email: "user@example.com"}
	if err := controller.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	code, err := controller.CreateUserOneTimeLoginCode(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.GetUserOneTimeLoginCode(user.Email, code); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.GetUserOneTimeLoginCode(user.Email, code); !errors.Is(err, lib.ErrInvalidToken) {
		t.Fatalf("second use error = %v, want ErrInvalidToken", err)
	}

	token, err := controller.CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if token.Token == "" || token.TokenHash != lib.HashToken(token.Token) {
		t.Fatal("returned token and stored hash do not match")
	}
	var stored lib.UserBearerToken
	if err := controller.DB.Where("token_hash = ?", token.TokenHash).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Token != "" || stored.TokenHash == "" || stored.ExpiresAt.Before(time.Now()) {
		t.Fatalf("token was not stored safely: plaintext=%q hash=%q", stored.Token, stored.TokenHash)
	}
}
