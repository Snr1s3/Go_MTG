package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Card struct {
	Name          string
	Quantity      int
	ScryfallID    string
	GameChanger   bool
	ManaCost      string
	Cmc           int
	TypeLine      string
	OracleText    string
	Colors        []string
	ColorIdentity []string
	Keywords      []string
	Power         string
	Toughness     string
}

type ScryfallCardResponse struct {
	GameChanger   bool     `json:"game_changer"`
	ManaCost      string   `json:"mana_cost"`
	Cmc           float64  `json:"cmc"`
	TypeLine      string   `json:"type_line"`
	OracleText    string   `json:"oracle_text"`
	Colors        []string `json:"colors"`
	ColorIdentity []string `json:"color_identity"`
	Keywords      []string `json:"keywords"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
}

func callAPI(client *http.Client, baseURL, id string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/cards/"+id, nil)
	if err != nil {
		return nil, err
	}

	// Scryfall requires both Accept and User-Agent headers on all requests.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-mtg/1.0 (+https://github.com/alba/go_mtg)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

// parseRow validates a CSV row and returns name, scryfallID, and quantity.
// Expects at least 9 columns: name at index 0, quantity at index 6, scryfallID at index 8.
func parseRow(fields []string) (name, scryfallID string, quantity int, err error) {
	if len(fields) <= 8 {
		return "", "", 0, fmt.Errorf("row has %d columns, need at least 9", len(fields))
	}
	quantity, err = strconv.Atoi(strings.TrimSpace(fields[6]))
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid quantity %q: %w", fields[6], err)
	}
	return fields[0], fields[8], quantity, nil
}

type cardResult struct {
	index int
	card  Card
	err   error
}

// cacheEntry ensures only one API call is made per unique Scryfall ID,
// even when multiple goroutines request the same ID concurrently.
type cacheEntry struct {
	once sync.Once
	resp ScryfallCardResponse
	err  error
}

func readCardsCSV(client *http.Client, baseURL, inputFile string, workers int) ([]Card, error) {
	f, err := os.Open(inputFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("csv header read error: %w", err)
	}

	var rows [][]string
	rowNumber := 1
	for {
		fields, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("csv row %d read error: %w", rowNumber+1, err)
		}
		rowNumber++
		rows = append(rows, fields)
	}

	cards := make([]Card, len(rows))
	var cacheMu sync.Mutex
	cache := make(map[string]*cacheEntry)
	sem := make(chan struct{}, workers)
	results := make(chan cardResult, len(rows))

	var wg sync.WaitGroup
	for i, fields := range rows {
		wg.Add(1)
		go func(idx int, fields []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name, scryfallID, quantity, err := parseRow(fields)
			if err != nil {
				slog.Error("row parse error", "row", idx+2, "error", err)
				results <- cardResult{index: idx, err: fmt.Errorf("row %d: %w", idx+2, err)}
				return
			}

			cacheMu.Lock()
			entry, ok := cache[scryfallID]
			if !ok {
				entry = &cacheEntry{}
				cache[scryfallID] = entry
			}
			cacheMu.Unlock()

			entry.once.Do(func() {
				body, err := callAPI(client, baseURL, scryfallID)
				if err != nil {
					entry.err = err
					return
				}
				if err := json.Unmarshal(body, &entry.resp); err != nil {
					entry.err = fmt.Errorf("json: %w", err)
				}
			})

			if entry.err != nil {
				slog.Error("api error", "row", idx+2, "scryfallID", scryfallID, "error", entry.err)
				results <- cardResult{index: idx, err: fmt.Errorf("row %d: %w", idx+2, entry.err)}
				return
			}
			apiData := entry.resp

			results <- cardResult{
				index: idx,
				card: Card{
					Name:          name,
					Quantity:      quantity,
					ScryfallID:    scryfallID,
					GameChanger:   apiData.GameChanger,
					ManaCost:      apiData.ManaCost,
					Cmc:           int(apiData.Cmc),
					TypeLine:      apiData.TypeLine,
					OracleText:    strings.ReplaceAll(apiData.OracleText, "\n", "\\n"),
					Colors:        apiData.Colors,
					ColorIdentity: apiData.ColorIdentity,
					Keywords:      apiData.Keywords,
					Power:         apiData.Power,
					Toughness:     apiData.Toughness,
				},
			}
			slog.Info("card fetched", "row", idx+2, "name", name, "scryfallID", scryfallID)
		}(i, fields)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		cards[res.index] = res.card
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return cards, nil
}

func writeCardsCSV(cards []Card, outputFile string) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	if err := writer.Write([]string{"Name", "Quantity", "Scryfall_ID", "Game Changer", "Mana_cost", "Cmc", "Type_line", "Oracle_text", "Colors", "Color_identity", "Keywords", "Power", "Toughness"}); err != nil {
		return fmt.Errorf("csv header write error: %w", err)
	}
	for _, card := range cards {
		if err := writer.Write([]string{
			card.Name,
			strconv.Itoa(card.Quantity),
			card.ScryfallID,
			strconv.FormatBool(card.GameChanger),
			card.ManaCost,
			strconv.Itoa(card.Cmc),
			card.TypeLine,
			card.OracleText,
			strings.Join(card.Colors, "|"),
			strings.Join(card.ColorIdentity, "|"),
			strings.Join(card.Keywords, "|"),
			card.Power,
			card.Toughness,
		}); err != nil {
			return fmt.Errorf("csv row write error for card %q: %w", card.Name, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("csv flush error: %w", err)
	}
	return nil
}

func buildS3ObjectKey(prefix, outputFile string, now time.Time) string {
	base := filepath.Base(outputFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "output"
	}

	key := fmt.Sprintf("%s_%s.csv", name, now.Format("20060102_150405"))
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return key
	}
	return prefix + "/" + key
}

func uploadFileToS3(ctx context.Context, localPath, bucket, key, region string) error {
	loadOpts := []func(*config.LoadOptions) error{}
	if strings.TrimSpace(region) != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer f.Close()

	client := s3.NewFromConfig(cfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String("text/csv"),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func main() {
	inputFile := flag.String("input", "Cartes.csv", "input CSV file")
	outputFile := flag.String("output", "output.csv", "output CSV file")
	apiBaseURL := flag.String("api", "https://api.scryfall.com", "Scryfall API base URL")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	workers := flag.Int("workers", 5, "number of concurrent API workers")
	s3Bucket := flag.String("s3-bucket", "go-mtg-card-bucket", "S3 bucket name (optional)")
	s3Prefix := flag.String("s3-prefix", "mtg/exports", "S3 key prefix (optional)")
	s3Region := flag.String("s3-region", "", "AWS region for S3 upload (optional; falls back to AWS config)")
	flag.Parse()

	slog.Info("starting", "input", *inputFile, "output", *outputFile, "api", *apiBaseURL, "workers", *workers)

	client := &http.Client{Timeout: *timeout}

	n, err := countLines(*inputFile)
	if err != nil {
		slog.Error("count lines error", "error", err)
		os.Exit(1)
	}
	slog.Info("lines counted", "count", n)

	cards, err := readCardsCSV(client, *apiBaseURL, *inputFile, *workers)
	if err != nil {
		slog.Error("read error", "error", err)
		os.Exit(1)
	}

	if err := writeCardsCSV(cards, *outputFile); err != nil {
		slog.Error("write error", "error", err)
		os.Exit(1)
	}

	if strings.TrimSpace(*s3Bucket) != "" {
		objectKey := buildS3ObjectKey(*s3Prefix, *outputFile, time.Now())
		if err := uploadFileToS3(context.Background(), *outputFile, *s3Bucket, objectKey, *s3Region); err != nil {
			slog.Error("s3 upload error", "bucket", *s3Bucket, "key", objectKey, "error", err)
			os.Exit(1)
		}
		slog.Info("s3 upload done", "bucket", *s3Bucket, "key", objectKey)
	}

	slog.Info("done", "cards", len(cards), "output", *outputFile)
}
