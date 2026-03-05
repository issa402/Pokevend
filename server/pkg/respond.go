// pkg/respond.go — shared HTTP response helpers used by all handlers
package pkg

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status code
func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Error writes a standard JSON error response
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
