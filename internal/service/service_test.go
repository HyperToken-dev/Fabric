package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
)

func newServiceMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	return db, mock, func() { _ = db.Close() }
}

func adminTestContext() context.Context {
	return adminauth.WithPrincipal(context.Background(), adminauth.Principal{
		OpenID:      "admin-openid",
		Role:        adminauth.RoleAdmin,
		Permissions: []string{adminauth.AdminPermission},
	})
}

func userTestContext() context.Context {
	return adminauth.WithPrincipal(context.Background(), adminauth.Principal{
		OpenID: "user-openid",
		Role:   adminauth.RoleUser,
	})
}
