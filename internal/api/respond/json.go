package respond

import (
	"encoding/json"
	"net/http"
)

func JSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func Error(writer http.ResponseWriter, status int, message string) {
	JSON(writer, status, map[string]string{"error": message})
}
