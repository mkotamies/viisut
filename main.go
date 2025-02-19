package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contestant struct {
	Id        string
	Name      string
	ViewCount string
	Updated   time.Time
	Event     string
	Country   *string
	VideoId   string
}

func getContestants(pool *pgxpool.Pool, eventP string) []Contestant {
	var contestants []Contestant
	rows, err := pool.Query(context.Background(), `SELECT
	ROW_NUMBER() OVER (ORDER BY s.view_count DESC) AS idx,
    c.name, s.view_count, s.updated, c.event, c.country FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id
    ORDER BY s.updated DESC
    LIMIT 1
) s ON true WHERE event = $1 ORDER BY s.view_count DESC`, eventP)

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var idx int64
		var name string
		var view_count int64
		var updated time.Time
		var event string
		var country *string

		err := rows.Scan(&idx, &name, &view_count, &updated, &event, &country)
		if err != nil {
			fmt.Println(err)
		}
		contestants = append(contestants, Contestant{Id: strconv.FormatInt(idx, 10), Name: name, ViewCount: humanize.Comma(view_count), Updated: updated, Country: country, Event: event})
	}
	return contestants
}

func getTimeInterval(pool *pgxpool.Pool) []string {
	var updateTimes []string
	query := `SELECT DISTINCT updated AS count FROM statistic ORDER BY updated ASC`

	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}

	for rows.Next() {
		var updated time.Time
		err := rows.Scan(&updated)
		if err != nil {
			fmt.Println(err)
		}
		updateTimes = append(updateTimes, updated.Format("2006-01-02"))

	}

	return updateTimes
}

type Views []struct {
	Count   int
	Updated time.Time
}
type ContestantViews struct {
	VideoId    string
	Name       string
	ViewCounts Views
}

func isInContestants(contestants []ContestantViews, videoId string) int {
	for i := range contestants {
		if (contestants)[i].VideoId == videoId {

			return i
		}
	}
	return -1
}

func AddOrUpdateContestantView(contestants []ContestantViews, videoId, name string, viewCount int, updated time.Time) []ContestantViews {
	if contestants == nil {
		contestants = append(contestants, ContestantViews{VideoId: videoId, Name: name, ViewCounts: Views{{Count: viewCount, Updated: updated}}})
		return contestants
	}

	var contenstantIndex = isInContestants(contestants, videoId)
	if contenstantIndex >= 0 {
		(contestants)[contenstantIndex].ViewCounts = append((contestants)[contenstantIndex].ViewCounts, struct {
			Count   int
			Updated time.Time
		}{
			Count:   viewCount,
			Updated: updated,
		})
	} else {
		contestants = append(contestants, ContestantViews{VideoId: videoId, Name: name, ViewCounts: Views{{Count: viewCount, Updated: updated}}})
	}
	return contestants
}

func getContestantViews(pool *pgxpool.Pool, event string) []ContestantViews {
	var contestantViews []ContestantViews
	rows, err := pool.Query(context.Background(), `SELECT
    c.video_id, c.name, s.view_count, s.updated FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id AND event = $1
    ORDER BY s.updated ASC
) s ON true WHERE c.video_id != 'PjthWPX1DcU'`, event)

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var name string
		var view_count int
		var updated time.Time
		var video_id string

		err := rows.Scan(&video_id, &name, &view_count, &updated)
		if err != nil {
			fmt.Println(err)
		}
		contestantViews = AddOrUpdateContestantView(contestantViews, video_id, name, view_count, updated)
	}
	return contestantViews
}

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

func main() {
	fmt.Println("Go app...")

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		http.ServeFile(w, r, "./favicon.ico")
	})

	dbpool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	go runDailyUpdate(dbpool)

	h1 := func(w http.ResponseWriter, r *http.Request) {
		event := "umk"
		wrappedWriter := &responseWriterWrapper{ResponseWriter: w}
		contestants := getContestants(dbpool, event)
		contestantsViews := getContestantViews(dbpool, event)
		var timeInterval = getTimeInterval(dbpool)

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

		tmpl := template.Must(template.ParseFiles("templates/index.html"))
		wrappedWriter.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := tmpl.Execute(wrappedWriter, data); err != nil {
			http.Error(wrappedWriter, err.Error(), http.StatusInternalServerError)
		}
	}

	eurovisionHandler := func(w http.ResponseWriter, r *http.Request) {
		event := "eurovision"
		contestants := getContestants(dbpool, event)

		type Contestant struct {
			Country   string `json:"country"`
			Name      string `json:"name"`
			Event     string `json:"event"`
			ViewCount string `json:"viewCount"`
		}
		contestantsTemp := []Contestant{
			{"Albania", "Shkodra Elektronike - Zjerm", "eurovision", "-"},
			{"Armenia", "Parg - Survivor", "eurovision", "-"},
			{"Australia", "", "eurovision", "-"},
			{"Austria", "JJ - Wasted Love", "eurovision", "-"},
			{"Azerbaijan", "Mamagama - Run with U", "eurovision", "-"},
			{"Belgium", "Red Sebastian - Strobe Lights", "eurovision", "-"},
			{"Croatia", "", "eurovision", "-"},
			{"Cyprus", "Theo Evan - ", "eurovision", "-"},
			{"Czechia", "Adonxs - Kiss Kiss Goodbye", "eurovision", "-"},
			{"Denmark", "", "eurovision", "-"},
			{"Estonia", "Tommy Cash - Espresso macchiato", "eurovision", "-"},
			{"Finland", "Erika Vikman - Ich komme", "eurovision", "-"},
			{"France", "Louane - ", "eurovision", "-"},
			{"Georgia", "", "eurovision", "-"},
			{"Germany", "", "eurovision", "-"},
			{"Greece", "Klavdia - Asteromata", "eurovision", "-"},
			{"Iceland", "", "eurovision", "-"},
			{"Ireland", "Emmy - Laika Party", "eurovision", "-"},
			{"Israel", "Yuval Raphael - ", "eurovision", "-"},
			{"Italy", "", "eurovision", "-"},
			{"Latvia", "Tautumeitas - Bur man laimi", "eurovision", "-"},
			{"Lithuania", "Katarsis - Tavo akys", "eurovision", "-"},
			{"Luxembourg", "Laura Thorn - La poupée monte le son", "eurovision", "-"},
			{"Malta", "Miriana Conte - Kant", "eurovision", "-"},
			{"Montenegro", "Nina Žižić - Dobrodošli", "eurovision", "-"},
			{"Netherlands", "Claude - ", "eurovision", "-"},
			{"Norway", "Kyle Alessandro - Lighter", "eurovision", "-"},
			{"Poland", "Justyna Steczkowska - Gaja", "eurovision", "-"},
			{"Portugal", "", "eurovision", "-"},
			{"San Marino", "", "eurovision", "-"},
			{"Serbia", "", "eurovision", "-"},
			{"Slovenia", "Klemen - How Much Time Do We Have Left", "eurovision", "-"},
			{"Spain", "Melody - Esa diva", "eurovision", "-"},
			{"Sweden", "", "eurovision", "-"},
			{"Switzerland", "", "eurovision", "-"},
			{"Ukraine", "Ziferblat - Bird of Pray", "eurovision", "-"},
			{"United Kingdom", "", "eurovision", "-"},
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

		tmpl := template.Must(template.ParseFiles("templates/euroviisut.html"))
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	http.HandleFunc("/euroviisut", eurovisionHandler)
	// define handlers
	http.HandleFunc("/", h1)

	log.Fatal(http.ListenAndServe(":9000", handlers.CompressHandler(http.DefaultServeMux)))

}
