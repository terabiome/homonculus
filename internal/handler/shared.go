package handler

import (
	"encoding/json"
	"net/http"
)

// GenericResponse is a standard API response structure
type GenericResponse struct {
	Body    any    `json:"body,omitempty"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// responseCallback is a function type for error handling callbacks
type responseCallback func()

// parseBodyAndHandleError parses the request body and handles errors
func parseBodyAndHandleError(writer http.ResponseWriter, request *http.Request, target any, requireBody bool) (responseCallback, error) {
	if requireBody {
		if err := json.NewDecoder(request.Body).Decode(target); err != nil {
			writeResult(writer, http.StatusBadRequest, GenericResponse{
				Body:    nil,
				Message: "invalid request body",
				Error:   err.Error(),
			})
			return func() {}, err
		}
	}
	return func() {}, nil
}

// writeResult writes a JSON response with the given status code
func writeResult(writer http.ResponseWriter, statusCode int, response GenericResponse) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	json.NewEncoder(writer).Encode(response)
}

// writeBytes writes raw bytes with the given status code
func writeBytes(writer http.ResponseWriter, statusCode int, data []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	writer.Write(data)
}
