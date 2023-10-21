package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

const (
	httpPort = 8080
	apiURL   = "https://economia.awesomeapi.com.br/json/last/USD-BRL"
)

type Exchange struct {
	USDBRL CambioUsdbrl `json:"USDBRL"`
}

type CambioUsdbrl struct {
	Code       string  `json:"code"`
	Codein     string  `json:"codein"`
	Name       string  `json:"name"`
	High       float64 `json:"high,string"`
	Low        float64 `json:"low,string"`
	VarBid     float64 `json:"varBid,string"`
	PctChange  float64 `json:"pctChange,string"`
	Bid        float64 `json:"bid,string"`
	Ask        float64 `json:"ask,string"`
	Timestamp  int64   `json:"timestamp,string"`
	CreateDate string  `json:"create_date"`
}

type Response struct {
	Bid float64 `json:"bid,string"`
}

func main() {
	db, err := initialiseDatabase()
	if err != nil {
		log.Fatalf("Error initialising the database: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/cotacao", handler(db))

	Logger("listening on %v", httpPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil))
}

func handler(db *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()

		Logger("%s %s %s", r.RemoteAddr, r.Method, r.URL)

		resp, err := doAPICall(ctx, apiURL)
		if err != nil {
			httpErrorResponse(w, err, http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		exchange, err := parseAPIResponse(resp)
		if err != nil {
			httpErrorResponse(w, err, http.StatusInternalServerError)
			return
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err = writeData(ctx, db, exchange.USDBRL)
		if err != nil {
			httpErrorResponse(w, err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Bid: exchange.USDBRL.Bid})
	}
}

func doAPICall(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("process timed out")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %v", resp.Status)
	}
	return resp, err
}

func parseAPIResponse(resp *http.Response) (*Exchange, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var exchange Exchange
	err = json.Unmarshal(data, &exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	return &exchange, nil
}

func writeData(ctx context.Context, db *sql.DB, data CambioUsdbrl) error {

	stmt, err := db.PrepareContext(ctx, "INSERT INTO CambioUsdbrl(Code, Codein, Name, High, Low, VarBid, PctChange, Bid, Ask, Timestamp, CreateDate) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, data.Code, data.Codein, data.Name, data.High, data.Low, data.VarBid, data.PctChange, data.Bid, data.Ask, data.Timestamp, data.CreateDate)
	if err != nil {
		return fmt.Errorf("failed to write to database: %v", err)
	}

	return nil
}

func initialiseDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./cambio.db")
	if err != nil {
		return nil, err
	}

	statements := `
	  DROP TABLE IF EXISTS CambioUsdbrl;
	  CREATE TABLE CambioUsdbrl
	      (
	          ID INTEGER PRIMARY KEY AUTOINCREMENT,
	          Code VARCHAR(3),
	          Codein VARCHAR(3),
	          Name VARCHAR(255),
	          High DECIMAL(10,4),
	          Low DECIMAL(10,4),
	          VarBid DECIMAL(10,4),
	          PctChange DECIMAL(10,2),
	          Bid DECIMAL(10,4),
	          Ask DECIMAL(10,4),
	          Timestamp BIGINT,
	          CreateDate DATETIME
	      );
	`
	_, err = db.Exec(statements)

	return db, err
}

func Logger(message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	log.Println(msg)
}

func httpErrorResponse(w http.ResponseWriter, err error, code int) {
	Logger("Error: %v", err)
	http.Error(w, fmt.Sprintf("Error: %v", err), code)
}
