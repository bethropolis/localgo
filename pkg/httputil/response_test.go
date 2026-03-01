package httputil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       interface{}
		wantSubstr string
	}{
		{
			name:       "empty object",
			statusCode: http.StatusOK,
			data:       struct{}{},
			wantSubstr: "{}",
		},
		{
			name:       "object with fields",
			statusCode: http.StatusOK,
			data:       map[string]string{"key": "value"},
			wantSubstr: `"key":"value"`,
		},
		{
			name:       "array",
			statusCode: http.StatusOK,
			data:       []int{1, 2, 3},
			wantSubstr: "[1,2,3]",
		},
		{
			name:       "string",
			statusCode: http.StatusOK,
			data:       "hello",
			wantSubstr: `"hello"`,
		},
		{
			name:       "integer",
			statusCode: http.StatusOK,
			data:       42,
			wantSubstr: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondJSON(w, tt.statusCode, tt.data)

			if w.Code != tt.statusCode {
				t.Errorf("RespondJSON status = %d; want %d", w.Code, tt.statusCode)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Content-Type = %s; want application/json", contentType)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.wantSubstr) {
				t.Errorf("Body = %s; want to contain %s", body, tt.wantSubstr)
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, http.StatusBadRequest, "test error message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("RespondError status = %d; want %d", w.Code, http.StatusBadRequest)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Content-Type = %s; want application/json", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test error message") {
		t.Errorf("Body = %s; want to contain 'test error message'", body)
	}
	if !strings.Contains(body, `"error"`) {
		t.Errorf("Body = %s; want to contain 'error' field", body)
	}
}

func TestRespondOK(t *testing.T) {
	w := httptest.NewRecorder()
	RespondOK(w)

	if w.Code != http.StatusOK {
		t.Errorf("RespondOK status = %d; want %d", w.Code, http.StatusOK)
	}
}
