package pins

import (
	"context"
	"errors"
	"testing"
)

type mockPinRepository struct {
	categories    []Category
	categoriesErr error
	pinDetail     PinDetail
	pinDetailErr  error
	pins          []PinListEntry
	pinsErr       error
	created       Pin
	createPinErr  error
	exists        bool
	existsErr     error
}

func (m *mockPinRepository) ListCategories(context.Context) ([]Category, error) {
	return m.categories, m.categoriesErr
}

func (m *mockPinRepository) CategoryExists(context.Context, int) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockPinRepository) GetPin(context.Context, string) (PinDetail, error) {
	return m.pinDetail, m.pinDetailErr
}

func (m *mockPinRepository) ListPins(context.Context, [4]float64, *int, int) ([]PinListEntry, error) {
	return m.pins, m.pinsErr
}

func (m *mockPinRepository) CreatePin(context.Context, NewPin) (Pin, error) {
	return m.created, m.createPinErr
}

func TestListCategories(t *testing.T) {
	svc := NewService(&mockPinRepository{
		categories: []Category{{ID: 1, Name: "Food"}},
	})
	categories, err := svc.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(categories) != 1 || categories[0].Name != "Food" {
		t.Fatalf("unexpected categories: %+v", categories)
	}
}

func TestGetPinPropagatesNotFound(t *testing.T) {
	svc := NewService(&mockPinRepository{pinDetailErr: ErrNotFound})
	_, err := svc.GetPin(context.Background(), "abc")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetPinSuccess(t *testing.T) {
	pin := PinDetail{Username: new(string)}
	svc := NewService(&mockPinRepository{pinDetail: pin})
	got, err := svc.GetPin(context.Background(), "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != pin.Username {
		t.Fatalf("unexpected pin detail: %+v", got)
	}
}

func TestCreatePinDelegates(t *testing.T) {
	created := Pin{Geohash: "ab12"}
	svc := NewService(&mockPinRepository{created: created})
	got, err := svc.CreatePin(context.Background(), NewPin{Geohash: "ab12"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Geohash != "ab12" {
		t.Fatalf("expected geohash ab12, got %q", got.Geohash)
	}
}

func TestCategoryExists(t *testing.T) {
	svc := NewService(&mockPinRepository{exists: true})
	exists, err := svc.CategoryExists(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected category to exist")
	}
}
