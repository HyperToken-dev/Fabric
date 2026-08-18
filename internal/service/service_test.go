package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
	"github.com/HyperToken-dev/fabric/internal/repository"
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
	return adminauth.WithUser(context.Background(), repository.User{ID: 99, Role: adminauth.RoleAdmin, Status: "active"})
}

func userTestContext() context.Context {
	return adminauth.WithUser(context.Background(), repository.User{ID: 77, Role: adminauth.RoleUser, Status: "active"})
}
