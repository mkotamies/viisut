package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5"
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

func GetContestantsFromDB(pool *pgxpool.Pool, event string) map[string][]Contestant {
	var contestants = make(map[string][]Contestant)
	rows, err := pool.Query(context.Background(), "select id, name, video_id from contestant WHERE event = $1", event)

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var id int64
		var name string
		var videoId string

		err := rows.Scan(&id, &name, &videoId)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(videoId)
		contestants["Contestants"] = append(contestants["Contestants"],
			Contestant{Id: strconv.FormatInt(id, 10), Name: name, VideoId: videoId})
	}
	return contestants
}

func InsertViewInfo(pool *pgxpool.Pool, contestantViews []VideoInfo) {
	var rows [][]any
	var updated = time.Now()

	for _, view := range contestantViews {
		var row []any
		row = append(row, view.Items[0].Statistics.ViewCount)
		row = append(row, view.VideoId)
		row = append(row, updated)
		rows = append(rows, row)
	}

	copyCount, copyError := pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"statistic"},
		[]string{"view_count", "video_id", "updated"},
		pgx.CopyFromRows(rows),
	)

	if copyError != nil {
		fmt.Println(copyError)
	}

	fmt.Println(copyCount)
}

func GetContestants(pool *pgxpool.Pool, eventP string) []Contestant {
	var contestants []Contestant
	rows, err := pool.Query(context.Background(), `SELECT
	ROW_NUMBER() OVER (ORDER BY s.view_count DESC) AS idx,
    c.name, COALESCE(s.view_count, 0), COALESCE(s.updated, NOW()), c.event, c.country FROM
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

func GetTimeInterval(pool *pgxpool.Pool) []string {
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

func AddOrUpdateContestantView(contestants map[string]ContestantViews, videoId, name string, viewCount int, updated time.Time) map[string]ContestantViews {
	contestant, exists := contestants[videoId]

	if exists {
		contestant.ViewCounts = append(contestant.ViewCounts, struct {
			Count   int
			Updated time.Time
		}{
			Count:   viewCount,
			Updated: updated,
		})

		contestants[videoId] = contestant
	} else {
		contestants[videoId] = ContestantViews{VideoId: videoId, Name: name, ViewCounts: Views{{Count: viewCount, Updated: updated}}}

	}

	return contestants
}

func GetContestantViews(pool *pgxpool.Pool, event string) map[string]ContestantViews {
	contestantViews := make(map[string]ContestantViews)

	rows, err := pool.Query(context.Background(), `SELECT
    c.video_id, c.name, s.view_count, s.updated FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id AND event = $1
    ORDER BY s.updated ASC
) s ON true WHERE c.video_id != 'PjthWPX1DcU' and event = $1`, event)

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
