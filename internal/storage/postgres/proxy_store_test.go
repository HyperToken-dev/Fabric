package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"fabric/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestProxyStoreResolveModel(t *testing.T) {
	tests := []struct {
		name       string
		status     int16
		wantStatus ModelStatus
	}{
		{name: "active", status: 1, wantStatus: ModelStatusActive},
		{name: "banned", status: 2, wantStatus: ModelStatusBanned},
		{name: "unknown", status: 9, wantStatus: ModelStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := newProxyStoreMock(t)
			defer cleanup()
			store := NewProxyStore(repository.New(db))

			mock.ExpectQuery(regexp.QuoteMeta("SELECT id, channel_id, model_name, model_type, status FROM models WHERE channel_id = $1 AND model_name = $2")).
				WithArgs(int32(10), "gpt-5.5").
				WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "model_name", "model_type", "status"}).
					AddRow(int32(99), int32(10), "gpt-5.5", int32(1), tt.status))

			model, err := store.ResolveModel(context.Background(), 10, " gpt-5.5 ")
			if err != nil {
				t.Fatal(err)
			}
			if model.ID != 99 || model.Status != tt.wantStatus {
				t.Fatalf("model = %+v, want ID=99 status=%v", model, tt.wantStatus)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProxyStoreResolveModelNoRows(t *testing.T) {
	db, mock, cleanup := newProxyStoreMock(t)
	defer cleanup()
	store := NewProxyStore(repository.New(db))

	mock.ExpectQuery("SELECT id, channel_id, model_name, model_type, status FROM models").
		WithArgs(int32(10), "missing").
		WillReturnError(sql.ErrNoRows)

	model, err := store.ResolveModel(context.Background(), 10, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if model != nil {
		t.Fatalf("model = %+v, want nil", model)
	}
}

func TestProxyStoreResolveModelQueryError(t *testing.T) {
	db, mock, cleanup := newProxyStoreMock(t)
	defer cleanup()
	store := NewProxyStore(repository.New(db))
	wantErr := errors.New("query failed")

	mock.ExpectQuery("SELECT id, channel_id, model_name, model_type, status FROM models").
		WithArgs(int32(10), "gpt-5.5").
		WillReturnError(wantErr)

	_, err := store.ResolveModel(context.Background(), 10, "gpt-5.5")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestProxyStoreInsertUsage(t *testing.T) {
	db, mock, cleanup := newProxyStoreMock(t)
	defer cleanup()
	store := NewProxyStore(repository.New(db))
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO usage_logs (key_id, channel_id, model_id, prompt_tokens, completion_tokens) VALUES ($1, $2, $3, $4, $5) RETURNING id, key_id, channel_id, model_id, prompt_tokens, completion_tokens, created_at")).
		WithArgs(int32(1), int32(2), int32(3), int64(4), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "channel_id", "model_id", "prompt_tokens", "completion_tokens", "created_at"}).
			AddRow(id, int32(1), int32(2), int32(3), int64(4), int64(5), now))

	if err := store.InsertUsage(context.Background(), 1, 2, 3, 4, 5); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProxyStoreInsertUsageError(t *testing.T) {
	db, mock, cleanup := newProxyStoreMock(t)
	defer cleanup()
	store := NewProxyStore(repository.New(db))
	wantErr := errors.New("insert failed")

	mock.ExpectQuery("INSERT INTO usage_logs").
		WithArgs(int32(1), int32(2), int32(3), int64(4), int64(5)).
		WillReturnError(wantErr)

	if err := store.InsertUsage(context.Background(), 1, 2, 3, 4, 5); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func newProxyStoreMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	return db, mock, func() {
		_ = db.Close()
	}
}
