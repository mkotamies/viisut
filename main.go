package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func generateLineItems(viewCounts Views) []int {
	items := make([]int, 0)
	for j := range viewCounts {
		items = append(items, viewCounts[j].Count)
	}
	return items
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

type responseWriterWrapper struct {
	http.ResponseWriter
	written bool
}

func (rw *responseWriterWrapper) WriteHeader(statusCode int) {
	if !rw.written {
		rw.ResponseWriter.WriteHeader(statusCode)
		rw.written = true
	}
}

func runDailyUpdate(pool *pgxpool.Pool) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Recovered from panic:", r)
				}
			}()

			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
			if now.After(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}

			sleepDuration := nextRun.Sub(now)

			timer := time.NewTimer(sleepDuration)
			<-timer.C

			go func() {
				fmt.Println("Running the scheduled task at", time.Now())

				UpdateContestantViews(pool, "eurovision")
			}()
		}()
	}
}

func umkHandler(w http.ResponseWriter, r *http.Request, dbpool *pgxpool.Pool) {
	event := "umk"
	wrappedWriter := &responseWriterWrapper{ResponseWriter: w}
	contestants := GetContestants(dbpool, "s.view_count", "DESC", event)
	contestantsViews := GetContestantViews(dbpool, event)
	var timeInterval = GetTimeInterval(dbpool, "2025-02-08 23:59:59")

	type ChartData struct {
		Label       string `json:"label"`
		Data        []int  `json:"data"`
		BorderWidth int    `json:"borderWidth"`
	}

	var chartData []ChartData

	for i := range contestantsViews {
		contestant := contestantsViews[i]
		chartData = append(chartData, ChartData{
			Label:       contestant.Name,
			Data:        generateLineItems(contestant.ViewCounts),
			BorderWidth: 1,
		})
	}

	data := struct {
		Contestants []Contestant
		Labels      string
		ChartData   string
	}{
		Contestants: contestants,
		Labels:      toJSON(timeInterval),
		ChartData:   toJSON(chartData),
	}

	tmpl := template.Must(template.ParseFiles("templates/umk.html", "templates/navbar.html", "templates/footer.html", "templates/scripts.html"))
	wrappedWriter.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tmpl.Execute(wrappedWriter, data); err != nil {
		http.Error(wrappedWriter, err.Error(), http.StatusInternalServerError)
	}
}

func eurovisionHandler(w http.ResponseWriter, r *http.Request, dbpool *pgxpool.Pool) {
	event := "eurovision"
	contestants := GetContestants(dbpool, "c.country", "ASC", event)

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl := template.Must(template.New("euroviisut.html").Funcs(funcMap).ParseFiles(
		"templates/euroviisut.html",
		"templates/navbar.html",
		"templates/footer.html",
		"templates/scripts.html",
	))

	data := struct {
		Contestants []Contestant
	}{
		Contestants: contestants,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html", "templates/navbar.html", "templates/footer.html", "templates/scripts.html"))
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handlerWithParam(originalHandler func(http.ResponseWriter, *http.Request, *pgxpool.Pool), pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		originalHandler(w, r, pool)
	}
}

func main() {
	fmt.Println("Go app...")

	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./robots.txt")
	})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		http.ServeFile(w, r, "./favicon.ico")
	})

	fs := http.FileServer(http.Dir("assets"))
	http.Handle("/assets/", http.StripPrefix("/assets/", fs))

	dbpool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	go runDailyUpdate(dbpool)

	http.HandleFunc("/euroviisut", handlerWithParam(eurovisionHandler, dbpool))
	http.HandleFunc("/umk", handlerWithParam(umkHandler, dbpool))
	http.HandleFunc("/", homeHandler)

	log.Fatal(http.ListenAndServe(":9000", handlers.CompressHandler(http.DefaultServeMux)))

}
