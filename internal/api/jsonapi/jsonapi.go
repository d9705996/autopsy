// Package jsonapi provides lightweight JSON:API 1.1 envelope types and
// rendering helpers. No external library is used — only encoding/json.
package jsonapi

import (
	"encoding/json"
	"net/http"
)

const contentType = "application/vnd.api+json"

// ---- Document types -------------------------------------------------------

// Document is a JSON:API single-resource document.
type Document struct {
	Data     any    `json:"data"`
	Included []any  `json:"included,omitempty"`
	Meta     Meta   `json:"meta,omitempty"`
	Links    *Links `json:"links,omitempty"`
}

// ListDocument is a JSON:API collection document.
type ListDocument struct {
	Data     []any       `json:"data"`
	Included []any       `json:"included,omitempty"`
	Meta     Meta        `json:"meta,omitempty"`
	Links    *Links      `json:"links,omitempty"`
	Paging   *Pagination `json:"page,omitempty"`
}

// ResourceObject is the canonical JSON:API resource object.
type ResourceObject struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    any                     `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Links         *Links                  `json:"links,omitempty"`
	Meta          Meta                    `json:"meta,omitempty"`
}

// Relationship represents a JSON:API relationship object.
type Relationship struct {
	Data  any    `json:"data,omitempty"`
	Links *Links `json:"links,omitempty"`
}

// Links holds JSON:API link objects.
type Links struct {
	Self    string `json:"self,omitempty"`
	Related string `json:"related,omitempty"`
	First   string `json:"first,omitempty"`
	Last    string `json:"last,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Next    string `json:"next,omitempty"`
}

// Meta is a free-form map of non-standard meta-information.
type Meta map[string]any

// Pagination holds JSON:API cursor-based pagination info.
type Pagination struct {
	Cursor   string `json:"cursor,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
	Total    int    `json:"total,omitempty"`
}

// ---- Error types ----------------------------------------------------------

// ErrorDocument is a JSON:API error response document.
type ErrorDocument struct {
	Errors []ErrorObject `json:"errors"`
}

// ErrorObject represents a single JSON:API error.
type ErrorObject struct {
	Status string       `json:"status,omitempty"`
	Code   string       `json:"code,omitempty"`
	Title  string       `json:"title,omitempty"`
	Detail string       `json:"detail,omitempty"`
	Source *ErrorSource `json:"source,omitempty"`
}

// ErrorSource identifies the source of a JSON:API error.
type ErrorSource struct {
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
}

// ---- Render helpers -------------------------------------------------------

// Render writes a JSON:API document to w with the given HTTP status code.
func Render(w http.ResponseWriter, status int, doc any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(doc)
}

// RenderOne writes a single-resource document.
func RenderOne(w http.ResponseWriter, status int, data any) {
	Render(w, status, Document{Data: data})
}

// RenderList writes a collection document.
func RenderList(w http.ResponseWriter, status int, data []any, pagination *Pagination) {
	if data == nil {
		data = []any{}
	}
	Render(w, status, ListDocument{Data: data, Paging: pagination})
}

// RenderError writes a single JSON:API error.
func RenderError(w http.ResponseWriter, status int, code, title, detail string) {
	RenderErrors(w, status, []ErrorObject{
		{
			Status: http.StatusText(status),
			Code:   code,
			Title:  title,
			Detail: detail,
		},
	})
}

// RenderErrors writes multiple JSON:API errors.
func RenderErrors(w http.ResponseWriter, status int, errs []ErrorObject) {
	Render(w, status, ErrorDocument{Errors: errs})
}
