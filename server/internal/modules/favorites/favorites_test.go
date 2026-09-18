package favorites

import (
	"context"
	"testing"
	"time"
)

type mockRepository struct {
	saveErr    error
	unsaveErr  error
	isSaved    bool
	isSavedErr error
	exists     bool
	existsErr  error
	entries    []Entry
	listErr    error
	ids        []string
	idsErr     error
}

func (m *mockRepository) Save(context.Context, string, string) error {
	return m.saveErr
}

func (m *mockRepository) Unsave(context.Context, string, string) error {
	return m.unsaveErr
}

func (m *mockRepository) IsSaved(context.Context, string, string) (bool, error) {
	return m.isSaved, m.isSavedErr
}

func (m *mockRepository) PinExists(context.Context, string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockRepository) List(context.Context, string) ([]Entry, error) {
	return m.entries, m.listErr
}

func (m *mockRepository) ListIDs(context.Context, string) ([]string, error) {
	return m.ids, m.idsErr
}

func TestSavePinDelegates(t *testing.T) {
	svc := NewService(&mockRepository{})
	err := svc.SavePin(context.Background(), "user-1", "pin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnsavePinDelegates(t *testing.T) {
	svc := NewService(&mockRepository{})
	err := svc.UnsavePin(context.Background(), "user-1", "pin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsSaved(t *testing.T) {
	svc := NewService(&mockRepository{isSaved: true})
	saved, err := svc.IsSaved(context.Background(), "user-1", "pin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !saved {
		t.Fatal("expected pin to be saved")
	}
}

func TestPinExists(t *testing.T) {
	svc := NewService(&mockRepository{exists: true})
	exists, err := svc.PinExists(context.Background(), "pin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected pin to exist")
	}
}

func TestListDelegates(t *testing.T) {
	entries := []Entry{{SavedAt: time.Now()}}
	svc := NewService(&mockRepository{entries: entries})
	got, err := svc.List(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
}

func TestListIDsDelegates(t *testing.T) {
	svc := NewService(&mockRepository{ids: []string{"pin-1"}})
	got, err := svc.ListIDs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "pin-1" {
		t.Fatalf("unexpected ids: %+v", got)
	}
}

func TestSavePinPropagatesError(t *testing.T) {
	svc := NewService(&mockRepository{saveErr: ErrNotFound})
	err := svc.SavePin(context.Background(), "user-1", "missing")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}