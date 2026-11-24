package converter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type PDFEngine interface {
	CountPages(ctx context.Context, path string) (int, error)

	RenderFile(
		ctx context.Context,
		input string,
		dpi int,
		outputForPage func(int) string,
		pageDone func(),
	) error
}

type Request struct {
	SourceDir       string `json:"sourceDir"`
	OutputDir       string `json:"outputDir"`
	DPI             int    `json:"dpi"`
	Organization    string `json:"organization"`
	PreserveFolders bool   `json:"preserveFolders"`
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.SourceDir) == "" {
		return errors.New("informe a pasta de origem")
	}
	if strings.TrimSpace(r.OutputDir) == "" {
		return errors.New("informe a pasta de destino")
	}

	switch r.DPI {
	case 100, 150, 200, 300:
	default:
		return errors.New("DPI deve ser 100, 150, 200 ou 300")
	}

	if r.Organization != "per-pdf" && r.Organization != "single" {
		return errors.New("organização de saída inválida")
	}

	return nil
}

type Status struct {
	Running        bool     `json:"running"`
	CurrentFile    string   `json:"currentFile"`
	ProcessedPDFs  int      `json:"processedPDFs"`
	TotalPDFs      int      `json:"totalPDFs"`
	ProcessedPages int      `json:"processedPages"`
	TotalPages     int      `json:"totalPages"`
	Message        string   `json:"message"`
	Errors         []string `json:"errors"`
}

type Manager struct {
	mu     sync.RWMutex
	engine PDFEngine
	status Status
	cancel context.CancelFunc
}

func NewManager(engine PDFEngine) *Manager {
	return &Manager{
		engine: engine,
		status: Status{
			Message: "Aguardando conversão",
		},
	}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := m.status
	status.Errors = append([]string(nil), m.status.Errors...)
	return status
}

func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) Start(req Request) error {
	if err := req.Validate(); err != nil {
		return err
	}

	source, err := filepath.Abs(filepath.Clean(req.SourceDir))
	if err != nil {
		return fmt.Errorf("pasta de origem inválida: %w", err)
	}

	output, err := filepath.Abs(filepath.Clean(req.OutputDir))
	if err != nil {
		return fmt.Errorf("pasta de destino inválida: %w", err)
	}

	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("não foi possível acessar a pasta de origem: %w", err)
	}
	if !info.IsDir() {
		return errors.New("a origem precisa ser uma pasta")
	}

	if samePath(source, output) {
		return errors.New("a pasta de destino não pode ser igual à pasta de origem")
	}

	if m.engine == nil {
		return errors.New("mecanismo PDFium não inicializado")
	}

	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta de destino: %w", err)
	}

	files, err := findPDFs(source, output)
	if err != nil {
		return fmt.Errorf("erro ao procurar PDFs: %w", err)
	}
	if len(files) == 0 {
		return errors.New("nenhum arquivo PDF encontrado na pasta de origem")
	}

	m.mu.Lock()
	if m.status.Running {
		m.mu.Unlock()
		return errors.New("já existe uma conversão em andamento")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.status = Status{
		Running:   true,
		TotalPDFs: len(files),
		Message:   "Preparando conversão",
	}
	m.mu.Unlock()

	go m.run(ctx, cancel, req, source, output, files)

	return nil
}

func (m *Manager) run(
	ctx context.Context,
	cancel context.CancelFunc,
	req Request,
	source string,
	output string,
	files []string,
) {
	defer func() {
		wasCancelled := ctx.Err() != nil
		cancel()

		m.mu.Lock()
		m.status.Running = false
		m.cancel = nil

		if wasCancelled {
			m.status.Message = "Conversão cancelada"
		} else if len(m.status.Errors) > 0 {
			m.status.Message = "Conversão concluída com erros"
		} else {
			m.status.Message = "Conversão concluída"
		}

		m.mu.Unlock()
	}()

	// Primeiro, contabiliza as páginas dos arquivos.
	for _, file := range files {
		if ctx.Err() != nil {
			return
		}

		pages, err := m.engine.CountPages(ctx, file)
		if err != nil {
			m.addError(fmt.Sprintf("%s: %v", file, err))
			continue
		}

		m.mu.Lock()
		m.status.TotalPages += pages
		m.mu.Unlock()
	}

	// Depois, realiza a conversão.
	for _, file := range files {
		if ctx.Err() != nil {
			return
		}

		m.mu.Lock()
		m.status.CurrentFile = filepath.Base(file)
		m.mu.Unlock()

		err := m.engine.RenderFile(
			ctx,
			file,
			req.DPI,
			func(page int) string {
				return outputPath(
					file,
					source,
					output,
					req,
					page,
				)
			},
			func() {
				m.mu.Lock()
				m.status.ProcessedPages++
				m.mu.Unlock()
			},
		)

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.addError(fmt.Sprintf("%s: %v", file, err))
		}

		m.mu.Lock()
		m.status.ProcessedPDFs++
		m.mu.Unlock()
	}
}

func (m *Manager) addError(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Errors = append(m.status.Errors, message)
}

func findPDFs(source, output string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path != source && entry.IsDir() {
			if samePath(path, output) {
				return filepath.SkipDir
			}
		}

		if entry.IsDir() {
			return nil
		}

		if strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func outputPath(
	file string,
	source string,
	output string,
	req Request,
	page int,
) string {
	relative, err := filepath.Rel(source, filepath.Dir(file))
	if err != nil {
		relative = "."
	}

	if !req.PreserveFolders {
		relative = "."
	}

	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	dir := filepath.Join(output, relative)

	if req.Organization == "per-pdf" {
		dir = filepath.Join(dir, base)
	}

	filename := fmt.Sprintf("pagina_%03d.png", page)

	if req.Organization == "single" {
		prefix := base

		if relative != "." {
			prefix = filepath.Join(relative, base)
			prefix = strings.ReplaceAll(prefix, string(filepath.Separator), "_")
		}

		filename = fmt.Sprintf("%s_pagina_%03d.png", prefix, page)
	}

	return filepath.Join(dir, filename)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)

	if runtimeGOOSWindows() {
		return strings.EqualFold(a, b)
	}

	return a == b
}

func runtimeGOOSWindows() bool {
	return os.PathSeparator == '\\'
}
