package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"pdfconverter/internal/converter"
)

func NewMux(assets embed.FS, jobManager *converter.Manager) *http.ServeMux {
	mux := http.NewServeMux()

	webFS, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}

	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// Consulta o progresso da conversão.
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, jobManager.Status())
	})

	// Inicia uma conversão.
	mux.HandleFunc("POST /api/convert", func(w http.ResponseWriter, r *http.Request) {
		var req converter.Request

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "JSON inválido: " + err.Error(),
			})
			return
		}

		if err := jobManager.Start(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]string{
			"message": "Conversão iniciada.",
		})
	})

	// Solicita o cancelamento.
	mux.HandleFunc("POST /api/cancel", func(w http.ResponseWriter, r *http.Request) {
		jobManager.Cancel()

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Solicitação de cancelamento recebida.",
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
