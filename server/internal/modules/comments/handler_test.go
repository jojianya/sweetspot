package comments

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

type fakeRepo struct {
	pinVisible bool
	createErr  error
}

func (f *fakeRepo) ListByPin(context.Context, string, int, int) ([]Comment, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Create(context.Context, string, string, string) (Comment, error) {
	return Comment{}, f.createErr
}
func (f *fakeRepo) Get(context.Context, string) (Comment, error) { return Comment{}, nil }
func (f *fakeRepo) Hide(context.Context, string) error           { return nil }
func (f *fakeRepo) Delete(context.Context, string) error         { return nil }
func (f *fakeRepo) PinExistsVisible(context.Context, string) (bool, error) {
	return f.pinVisible, nil
}

func commentRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/pins/p1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestCreateMapsFKViolationTo404 proves the pg 23503 safety net: if the pin
// disappears between the visibility check and the insert, the caller gets
// a 404, never a 500.
func TestCreateMapsFKViolationTo404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&fakeRepo{pinVisible: true, createErr: &pgconn.PgError{Code: "23503"}}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = commentRequest(http.MethodPost, `{"body":"hi"}`)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	c.Set(middleware.CtxUserID, "u1")

	h.Create(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%q)", w.Code, w.Body.String())
	}
}

// TestCreateOtherErrorsStill500 makes sure only FK violations are remapped.
func TestCreateOtherErrorsStill500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&fakeRepo{pinVisible: true, createErr: errors.New("db down")}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = commentRequest(http.MethodPost, `{"body":"hi"}`)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	c.Set(middleware.CtxUserID, "u1")

	h.Create(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body=%q)", w.Code, w.Body.String())
	}
}

// TestListHiddenPinReturns404 proves listing comments on a hidden pin 404s.
func TestListHiddenPinReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&fakeRepo{pinVisible: false}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = commentRequest(http.MethodGet, "")
	c.Params = gin.Params{{Key: "id", Value: "p1"}}

	h.List(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%q)", w.Code, w.Body.String())
	}
}

// TestCreateHiddenPinReturns404 proves creating on a hidden pin 404s before
// ever hitting the insert.
func TestCreateHiddenPinReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&fakeRepo{pinVisible: false}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = commentRequest(http.MethodPost, `{"body":"hi"}`)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	c.Set(middleware.CtxUserID, "u1")

	h.Create(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%q)", w.Code, w.Body.String())
	}
}
