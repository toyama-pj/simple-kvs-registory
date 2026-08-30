package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func TestNamespaceMemberManagementRequiresAdmin(t *testing.T) {
	db := resourceTestDB(t)
	admin := lib.User{ID: uuid.New(), Name: "Admin", Email: "admin-members@example.com"}
	reader := lib.User{ID: uuid.New(), Name: "Reader", Email: "reader-members@example.com"}
	target := lib.User{ID: uuid.New(), Name: "Target", Email: "target-members@example.com"}
	for _, user := range []lib.User{admin, reader, target} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	libController := lib.NewController(db, lib.Config{})
	namespaceID, err := libController.CreateNamespace(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := libController.PermitUserToAccessNamespace(admin.ID.String(), reader.ID.String(), namespaceID.String(), "r"); err != nil {
		t.Fatal(err)
	}
	adminToken, err := libController.CreateUserBearerToken(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	readerToken, err := libController.CreateUserBearerToken(reader.ID)
	if err != nil {
		t.Fatal(err)
	}

	controller := NewController(db, lib.Config{})
	app := fiber.New()
	app.Route("/api/v1/cfg/", controller.CfgHandlersSetup)
	memberPath := "/api/v1/cfg/" + namespaceID.String()

	forbidden := cfgMemberRequest(t, app, http.MethodGet, memberPath+"/members", readerToken.Token, "")
	if forbidden.StatusCode != fiber.StatusForbidden {
		t.Fatalf("reader member list = %d", forbidden.StatusCode)
	}
	readerInvite := cfgMemberRequest(t, app, http.MethodPost, memberPath+"/invite", readerToken.Token, `{"email":"target-members@example.com","grant_type":"rw"}`)
	if readerInvite.StatusCode != fiber.StatusForbidden {
		t.Fatalf("reader invite = %d", readerInvite.StatusCode)
	}

	invite := cfgMemberRequest(t, app, http.MethodPost, memberPath+"/invite", adminToken.Token, `{"email":"target-members@example.com","grant_type":"rw"}`)
	if invite.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(invite.Body)
		t.Fatalf("admin invite = %d: %s", invite.StatusCode, body)
	}
	list := cfgMemberRequest(t, app, http.MethodGet, memberPath+"/members", adminToken.Token, "")
	if list.StatusCode != fiber.StatusOK {
		t.Fatalf("admin member list = %d", list.StatusCode)
	}
	var members NamespaceMembersResponse
	if err := json.NewDecoder(list.Body).Decode(&members); err != nil {
		t.Fatal(err)
	}
	if len(members.Data) != 3 {
		t.Fatalf("member count = %d, want 3", len(members.Data))
	}
	foundTarget := false
	for _, member := range members.Data {
		if member.UserID == target.ID {
			foundTarget = member.Email == target.Email && member.GrantType == "rw"
		}
	}
	if !foundTarget {
		t.Fatalf("target member missing: %#v", members.Data)
	}

	selfDemotion := cfgMemberRequest(t, app, http.MethodPost, memberPath+"/invite", adminToken.Token, `{"email":"admin-members@example.com","grant_type":"r"}`)
	if selfDemotion.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("self demotion = %d", selfDemotion.StatusCode)
	}
	selfRemoval := cfgMemberRequest(t, app, http.MethodPost, memberPath+"/disinvite", adminToken.Token, `{"email":"admin-members@example.com"}`)
	if selfRemoval.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("self removal = %d", selfRemoval.StatusCode)
	}
	remove := cfgMemberRequest(t, app, http.MethodPost, memberPath+"/disinvite", adminToken.Token, `{"email":"target-members@example.com"}`)
	if remove.StatusCode != fiber.StatusNoContent {
		t.Fatalf("remove target = %d", remove.StatusCode)
	}
}

func cfgMemberRequest(t *testing.T, app *fiber.App, method, path, token, body string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
