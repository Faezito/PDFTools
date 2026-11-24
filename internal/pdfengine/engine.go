package pdfengine

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

type Engine struct {
	instance pdfium.Pdfium
	mu       sync.Mutex
}

func New() (*Engine, error) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("iniciar PDFium: %w", err)
	}

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("obter instância PDFium: %w", err)
	}

	return &Engine{instance: instance}, nil
}

func (e *Engine) CountPages(ctx context.Context, path string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	doc, err := e.instance.OpenDocument(&requests.OpenDocument{
		File: &data,
	})
	if err != nil {
		return 0, err
	}

	defer e.instance.FPDF_CloseDocument(
		&requests.FPDF_CloseDocument{
			Document: doc.Document,
		},
	)

	result, err := e.instance.FPDF_GetPageCount(
		&requests.FPDF_GetPageCount{
			Document: doc.Document,
		},
	)
	if err != nil {
		return 0, err
	}

	return result.PageCount, nil
}

func (e *Engine) RenderFile(
	ctx context.Context,
	input string,
	dpi int,
	outputForPage func(int) string,
	pageDone func(),
) error {

	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	doc, err := e.instance.OpenDocument(
		&requests.OpenDocument{File: &data},
	)
	if err != nil {
		return err
	}

	defer e.instance.FPDF_CloseDocument(
		&requests.FPDF_CloseDocument{
			Document: doc.Document,
		},
	)

	count, err := e.instance.FPDF_GetPageCount(
		&requests.FPDF_GetPageCount{
			Document: doc.Document,
		},
	)
	if err != nil {
		return err
	}

	for i := 0; i < count.PageCount; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		rendered, err := e.instance.RenderPageInDPI(
			&requests.RenderPageInDPI{
				DPI: dpi,
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{
						Document: doc.Document,
						Index:    i,
					},
				},
			},
		)
		if err != nil {
			return fmt.Errorf("página %d: %w", i+1, err)
		}

		err = func() error {
			defer rendered.Cleanup()

			output := outputForPage(i + 1)

			if err := os.MkdirAll(
				filepath.Dir(output), 0755,
			); err != nil {
				return err
			}

			file, err := os.Create(output)
			if err != nil {
				return err
			}
			defer file.Close()

			return png.Encode(file, rendered.Result.Image)
		}()

		if err != nil {
			return fmt.Errorf("salvar página %d: %w", i+1, err)
		}

		if pageDone != nil {
			pageDone()
		}
	}

	return nil
}
