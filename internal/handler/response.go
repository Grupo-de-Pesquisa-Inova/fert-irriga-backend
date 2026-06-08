package handler

import (
	"encoding/json"
	"net/http"
)

// respondJSON serializa data como JSON e escreve na resposta HTTP.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// respondError escreve uma resposta de erro padronizada.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
