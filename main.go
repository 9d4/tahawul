package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

func main() {
	r := chi.NewRouter()
	r.Post("/json", func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(400)
			return
		}

		excelFile, err := excelize.OpenReader(file)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		sheets := excelFile.GetSheetList()

		querySheet := r.URL.Query().Get("sheet")
		if querySheet != "" && slices.Contains(sheets, querySheet) {
			sheets = []string{querySheet}
		}

		type sheetResult struct {
			sheet string
			data  any
			err   error
		}

		ctx, cancel := context.WithCancel(context.Background())
		results := make(chan sheetResult, len(sheets))
		wg := sync.WaitGroup{}

		wg.Add(len(sheets))
		for _, sheet := range sheets {
			sheet := sheet
			go func() {
				data, err := readSheet(ctx, excelFile, sheet)
				if err != nil {
					results <- sheetResult{
						err: err,
					}
					return
				}
				results <- sheetResult{
					sheet: sheet,
					data:  data,
				}
				wg.Done()
			}()
		}

		go func() {
			wg.Wait()
			close(results)
			cancel()
		}()

		out := map[string]any{}
		for res := range results {
			if res.err != nil {
				w.WriteHeader(500)
				log.Err(err).Send()
				cancel()
				return
			}
			out[res.sheet] = res.data
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(out); err != nil {
			log.Err(err).Send()
			w.WriteHeader(500)
		}
	})

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().Msgf("server listening on %s", ln.Addr().String())

	if err := http.Serve(ln, r); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func readSheet(ctx context.Context, excelFile *excelize.File, sheet string) ([]map[string]any, error) {
	rows, err := excelFile.Rows(sheet)
	if err != nil {
		return nil, err
	}

	headers := []string{}
	sheetOut := []map[string]any{}

	index := 0
	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if index == 0 {
			cols, err := rows.Columns()
			if err != nil {
				return nil, err
			}

			for _, v := range cols {
				v = strings.TrimSpace(v)
				v = strings.ToLower(v)
				v = strings.ReplaceAll(v, " ", "_")
				headers = append(headers, v)
			}

			index++
			continue
		}

		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		currentMap := map[string]any{}
		for _, v := range headers {
			currentMap[v] = nil
		}
		for i, v := range cols {
			if i > len(headers)-1 {
				continue
			}
			currentMap[headers[i]] = v
		}
		sheetOut = append(sheetOut, currentMap)
		index++
	}

	return sheetOut, nil
}
