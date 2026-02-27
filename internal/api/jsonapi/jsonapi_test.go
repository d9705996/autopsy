package jsonapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/d9705996/autopsy/internal/api/jsonapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderOne(t *testing.T) {
	type attrs struct {
		Name string `json:"name"`
	}

	w := httptest.NewRecorder()
	jsonapi.RenderOne(w, http.StatusOK, jsonapi.ResourceObject{
		Type:       "widgets",
		ID:         "1",
		Attributes: attrs{Name: "test"},
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

	var doc jsonapi.Document
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.NotNil(t, doc.Data)
}

func TestRenderList_EmptySlice(t *testing.T) {
	w := httptest.NewRecorder()
	jsonapi.RenderList(w, http.StatusOK, nil, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var doc jsonapi.ListDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.NotNil(t, doc.Data)
	assert.Len(t, doc.Data, 0)
}

func TestRenderError(t *testing.T) {
	w := httptest.NewRecorder()
	jsonapi.RenderError(w, http.StatusNotFound, "not_found", "Not Found", "the resource does not exist")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var doc jsonapi.ErrorDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	require.Len(t, doc.Errors, 1)
	assert.Equal(t, "not_found", doc.Errors[0].Code)
	assert.Equal(t, "the resource does not exist", doc.Errors[0].Detail)
}

func TestRenderErrors_MultipleErrors(t *testing.T) {
	w := httptest.NewRecorder()
	jsonapi.RenderErrors(w, http.StatusUnprocessableEntity, []jsonapi.ErrorObject{
		{
			Code: "missing_field", Title: "Missing Field", Detail: "name is required",
			Source: &jsonapi.ErrorSource{Pointer: "/data/attributes/name"},
		},
		{
			Code: "missing_field", Title: "Missing Field", Detail: "email is required",
			Source: &jsonapi.ErrorSource{Pointer: "/data/attributes/email"},
		},
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var doc jsonapi.ErrorDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.Len(t, doc.Errors, 2)
}
