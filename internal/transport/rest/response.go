package rest

import (
	"encoding/json"
	"log"
	"net/http"
)

type APIResponse struct {
	ErrorCode int         `json:"error_code"`
	Status    string      `json:"status"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
}

func Response(w http.ResponseWriter, message string, data interface{}, errorCode int, status string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	response := APIResponse{
		ErrorCode: errorCode,
		Status:    status,
		Message:   message,
		Data:      data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[HTTP] write response error: %v", err)
	}
}

func Success(w http.ResponseWriter, message string, data interface{}) {
	Response(w, message, data, 0, "success", http.StatusOK)
}

func SuccessAccepted(w http.ResponseWriter, message string, data interface{}) {
	Response(w, message, data, 0, "success", http.StatusAccepted)
}

func Error(w http.ResponseWriter, message string, errorCode int, httpStatus int) {
	Response(w, message, nil, errorCode, "error", httpStatus)
}

func ErrorBadRequest(w http.ResponseWriter, message string) {
	Error(w, message, 400, http.StatusBadRequest)
}

func ErrorUnauthorized(w http.ResponseWriter, message string) {
	Error(w, message, 401, http.StatusUnauthorized)
}

func ErrorNotFound(w http.ResponseWriter, message string) {
	Error(w, message, 404, http.StatusNotFound)
}

func ErrorInternal(w http.ResponseWriter, message string) {
	Error(w, message, 500, http.StatusInternalServerError)
}
