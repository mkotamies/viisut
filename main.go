package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/jackc/pgx/v5"
)

type Contestant struct {
	Id        string
	Name      string
	ViewCount int
	Updated   time.Time
}

func getContestants(conn *pgx.Conn) map[string][]Contestant {
	var contestants = make(map[string][]Contestant)
	rows, err := conn.Query(context.Background(), `SELECT
	ROW_NUMBER() OVER (ORDER BY updated DESC) AS idx,
    c.name, s.view_count, s.updated FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id
    ORDER BY s.updated DESC
    LIMIT 1
) s ON true ORDER BY s.view_count DESC`)

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var idx int32
		var name string
		var view_count int32
		var updated time.Time

		err := rows.Scan(&idx, &name, &view_count, &updated)
		if err != nil {
			fmt.Println(err)
		}
		contestants["Contestants"] = append(contestants["Contestants"], Contestant{Id: strconv.FormatInt(int64(idx), 10), Name: name, ViewCount: int(view_count), Updated: updated})
	}
	return contestants
}

func getTimeInterval(conn *pgx.Conn) []string {
	var updateTimes []string
	query := `SELECT DISTINCT updated AS count FROM statistic`

	rows, err := conn.Query(context.Background(), query)
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

func AddOrUpdateContestantView(contestants []ContestantViews, videoId, name string, viewCount int, updated time.Time) []ContestantViews {
	if contestants == nil {
		contestants = append(contestants, ContestantViews{VideoId: videoId, Name: name, ViewCounts: Views{{Count: viewCount, Updated: updated}}})
		return contestants
	}
	for i := range contestants {
		if (contestants)[i].VideoId == videoId {
			(contestants)[i].ViewCounts = append((contestants)[i].ViewCounts, struct {
				Count   int
				Updated time.Time
			}{
				Count:   viewCount,
				Updated: updated,
			})
		}
		contestants = append(contestants, ContestantViews{VideoId: videoId, Name: name, ViewCounts: Views{{Count: viewCount, Updated: updated}}})
	}
	return contestants
}

func getContestantViews(conn *pgx.Conn) []ContestantViews {
	var contestantViews []ContestantViews
	rows, err := conn.Query(context.Background(), `SELECT
    c.video_id, c.name, s.view_count, s.updated FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id
    ORDER BY s.updated ASC
) s ON true`)

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

func generateLineItems(viewCounts Views) []opts.LineData {
	items := make([]opts.LineData, 0)
	for j := range viewCounts {
		items = append(items, opts.LineData{Value: viewCounts[j].Count})
	}
	return items
}

func httpserver(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var timeInterval = getTimeInterval(conn)
		var contestantViews = getContestantViews(conn)

		line := charts.NewLine()
		line.SetGlobalOptions(
			charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),
			charts.WithTitleOpts(opts.Title{
				Title:    "Video trends over time",
				Subtitle: "Video views",
			}))

		var xAxis = line.SetXAxis(timeInterval)

		for i := range contestantViews {
			var contestant = contestantViews[i]
			xAxis.AddSeries(contestant.Name, generateLineItems(contestant.ViewCounts))
		}
		xAxis.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: opts.Bool(true)}))
		line.Render(w)
	}
}

func main() {
	fmt.Println("Go app...")

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var contestants map[string][]Contestant

	h1 := func(w http.ResponseWriter, r *http.Request) {
		contestants = getContestants(conn)
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, contestants)
	}

	// define handlers
	http.HandleFunc("/", h1)
	http.HandleFunc("/chart", httpserver(conn))

	log.Fatal(http.ListenAndServe("localhost:9000", nil))

}
