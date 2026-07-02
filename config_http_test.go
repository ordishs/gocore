package gocore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleConfig(t *testing.T) {
	Config().Get("name")

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()

	HandleConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "id='settingsTable'")
	assert.Contains(t, body, "id='requestedTable'")
	assert.Contains(t, body, "GoCore Configuration")
	assert.Contains(t, body, "Requests")
	assert.Contains(t, body, "name")
}
