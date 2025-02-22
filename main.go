package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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
	contestants := GetContestants(dbpool, event)
	contestantsViews := GetContestantViews(dbpool, event)
	var timeInterval = GetTimeInterval(dbpool)

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
	contestants := GetContestants(dbpool, event)

	type Contestant struct {
		Country     string `json:"country"`
		Name        string `json:"name"`
		Event       string `json:"event"`
		ViewCount   string `json:"viewCount"`
		CountryCode string `json:"countryCode"`
	}
	contestantsTemp := []Contestant{
		{"Albania", "Shkodra Elektronike - Zjerm", "eurovision", "-", "al"},
		{"Armenia", "Parg - Survivor", "eurovision", "-", "am"},
		{"Australia", "", "eurovision", "-", "au"},
		{"Austria", "JJ - Wasted Love", "eurovision", "-", "at"},
		{"Azerbaijan", "Mamagama - Run with U", "eurovision", "-", "az"},
		{"Belgium", "Red Sebastian - Strobe Lights", "eurovision", "-", "be"},
		{"Croatia", "", "eurovision", "-", "hr"},
		{"Cyprus", "Theo Evan - ", "eurovision", "-", "cy"},
		{"Czechia", "Adonxs - Kiss Kiss Goodbye", "eurovision", "-", "cz"},
		{"Denmark", "", "eurovision", "-", "dk"},
		{"Estonia", "Tommy Cash - Espresso macchiato", "eurovision", "-", "ee"},
		{"Finland", "Erika Vikman - Ich komme", "eurovision", "-", "fi"},
		{"France", "Louane - ", "eurovision", "-", "fr"},
		{"Georgia", "", "eurovision", "-", "ge"},
		{"Germany", "", "eurovision", "-", "de"},
		{"Greece", "Klavdia - Asteromata", "eurovision", "-", "gr"},
		{"Iceland", "", "eurovision", "-", "is"},
		{"Ireland", "Emmy - Laika Party", "eurovision", "-", "ie"},
		{"Israel", "Yuval Raphael - ", "eurovision", "-", "il"},
		{"Italy", "", "eurovision", "-", "it"},
		{"Latvia", "Tautumeitas - Bur man laimi", "eurovision", "-", "lv"},
		{"Lithuania", "Katarsis - Tavo akys", "eurovision", "-", "lt"},
		{"Luxembourg", "Laura Thorn - La poupée monte le son", "eurovision", "-", "lu"},
		{"Malta", "Miriana Conte - Kant", "eurovision", "-", "mt"},
		{"Montenegro", "Nina Žižić - Dobrodošli", "eurovision", "-", "me"},
		{"Netherlands", "Claude - ", "eurovision", "-", "nl"},
		{"Norway", "Kyle Alessandro - Lighter", "eurovision", "-", "no"},
		{"Poland", "Justyna Steczkowska - Gaja", "eurovision", "-", "pl"},
		{"Portugal", "", "eurovision", "-", "pt"},
		{"San Marino", "", "eurovision", "-", "sm"},
		{"Serbia", "", "eurovision", "-", "rs"},
		{"Slovenia", "Klemen - How Much Time Do We Have Left", "eurovision", "-", "si"},
		{"Spain", "Melody - Esa diva", "eurovision", "-", "es"},
		{"Sweden", "", "eurovision", "-", "se"},
		{"Switzerland", "", "eurovision", "-", "ch"},
		{"Ukraine", "Ziferblat - Bird of Pray", "eurovision", "-", "ua"},
		{"United Kingdom", "", "eurovision", "-", "gb"},
	}

	for i := range contestantsTemp {
		for j := range contestants {
			if contestants[j].Name == contestantsTemp[i].Name {
				contestantsTemp[i].ViewCount = contestants[j].ViewCount
			}
		}
	}

	data := struct {
		Contestants []Contestant
	}{
		Contestants: contestantsTemp,
	}

	tmpl := template.Must(template.ParseFiles("templates/euroviisut.html", "templates/navbar.html", "templates/footer.html", "templates/scripts.html"))
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
