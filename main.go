package main

import (
	"embed"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"pdfconverter/internal/converter"
	"pdfconverter/internal/pdfengine"
	"pdfconverter/internal/server"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	// Inicializa o mecanismo PDFium.
	engine, err := pdfengine.New()
	if err != nil {
		log.Fatalf("não foi possível inicializar o PDFium: %v", err)
	}

	// Cria o gerenciador de conversões.
	manager := converter.NewManager(engine)

	// Conecta a API ao gerenciador.
	mux := server.NewMux(webFiles, manager)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("não foi possível iniciar o servidor local: %v", err)
	}

	addr := "http://" + listener.Addr().String()

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Interface disponível em %s", addr)
		if err := openBrowser(addr); err != nil {
			log.Printf("Não foi possível abrir o navegador automaticamente: %v", err)
			log.Printf("Abra manualmente: %s", addr)
		}
	}()

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}
