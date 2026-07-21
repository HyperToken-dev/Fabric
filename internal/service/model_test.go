package service

import (
	"context"
	"regexp"
	"testing"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestModelServiceGetCreateAndList(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewModelService(db)
	modelRows := []string{"id", "channel_id", "model_name", "model_type", "status"}

	mock.ExpectQuery("SELECT id, channel_id, model_name, model_type, status FROM models WHERE id").
		WithArgs(int32(9)).
		WillReturnRows(sqlmock.NewRows(modelRows).AddRow(int32(9), int32(7), "gpt-5.5", int32(1), int16(1)))
	got, err := svc.GetModelInfo(context.Background(), &proto.GetModelInfoRequest{ModelId: 9})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model.ModelId != 9 || got.Model.ModelName != "gpt-5.5" {
		t.Fatalf("model = %+v", got.Model)
	}

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO models (channel_id, model_name, status, model_type) VALUES ($1, $2, $3, $4) RETURNING id, channel_id, model_name, model_type, status")).
		WithArgs(int32(7), "gpt-5.5-mini", int16(modelStatusActive), int32(modelTypeText)).
		WillReturnRows(sqlmock.NewRows(modelRows).AddRow(int32(10), int32(7), "gpt-5.5-mini", int32(modelTypeText), int16(modelStatusActive)))
	created, err := svc.CreateModel(context.Background(), &proto.CreateModelRequest{ChannelId: 7, ModelName: "gpt-5.5-mini"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Model.Status != int32(modelStatusActive) || created.Model.ModelType != modelTypeText {
		t.Fatalf("created model = %+v", created.Model)
	}

	mock.ExpectQuery("SELECT id, channel_id, model_name, model_type, status FROM models WHERE channel_id").
		WithArgs(int32(7)).
		WillReturnRows(sqlmock.NewRows(modelRows).AddRow(int32(11), int32(7), "gpt-5.5", int32(1), int16(1)))
	list, err := svc.ListModels(context.Background(), &proto.ListModelsRequest{ChannelId: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Models) != 1 || list.Models[0].ModelId != 11 {
		t.Fatalf("models = %+v", list.Models)
	}

	catalog, err := svc.ListCatalogModels(context.Background(), &proto.ListCatalogModelsRequest{ApiFormat: models.APIFormatOpenAI})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) == 0 || catalog.Models[0].ModelName == "" || catalog.Models[0].ModelType != modelTypeText {
		t.Fatalf("catalog models = %+v", catalog.Models)
	}

	emptyCatalog, err := svc.ListCatalogModels(context.Background(), &proto.ListCatalogModelsRequest{ApiFormat: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyCatalog.Models) != 0 {
		t.Fatalf("unsupported catalog models = %+v", emptyCatalog.Models)
	}
}
