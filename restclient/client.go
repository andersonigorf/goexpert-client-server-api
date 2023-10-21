package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	urlCotacaoServer = "http://localhost:8080/cotacao"
	OutputFile       = "cotacao.txt"
)

func main() {
	err := fetchAndSaveData()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func fetchAndSaveData() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	respBody, err := makeGetRequest(ctx, urlCotacaoServer)
	if err != nil {
		return err
	}
	var result map[string]string
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return fmt.Errorf("erro json: %v", err)
	}

	if _, ok := result["bid"]; !ok {
		return fmt.Errorf("JSON does not contain 'bid' field")
	}

	newData := []byte(fmt.Sprintf("Dólar: %s", result["bid"]))

	_, err = writeDataToFile(OutputFile, newData)
	if err != nil {
		return err
	}

	Logger("request processed with output file: ./%v", OutputFile)

	return nil
}

func makeGetRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(req)

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("process timed out")
		}
		return nil, err
	}

	return res, nil
}

func doRequest(req *http.Request) ([]byte, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %v", res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func writeDataToFile(filename string, data []byte) (int, error) {
	f, err := os.Create(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := f.Write(data)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func Logger(message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	log.Println(msg)
}
