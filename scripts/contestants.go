package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type VideoInfo struct {
	VideoId string `json:"videoId"`
	Kind    string `json:"kind"`
	Etag    string `json:"etag"`
	Items   []struct {
		Kind       string `json:"kind"`
		Etag       string `json:"etag"`
		ID         string `json:"id"`
		Statistics struct {
			ViewCount     string `json:"viewCount"`
			LikeCount     string `json:"likeCount"`
			FavoriteCount string `json:"favoriteCount"`
			CommentCount  string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
}

type Contestant struct {
	Id      string
	Name    string
	VideoId string
}

func getContestantsFromDB(conn *pgx.Conn) map[string][]Contestant {
	var contestants = make(map[string][]Contestant)
	rows, err := conn.Query(context.Background(), "select id, name, video_id from contestant")

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var id int32
		var name string
		var videoId string

		err := rows.Scan(&id, &name, &videoId)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(videoId)
		contestants["Contestants"] = append(contestants["Contestants"],
			Contestant{Id: strconv.FormatInt(int64(id), 10), Name: name, VideoId: videoId})
	}
	return contestants
}

func insertViewInfo(conn *pgx.Conn, contestantViews []VideoInfo) {
	var rows [][]any
	var updated = time.Now()

	for _, view := range contestantViews {
		var row []any
		row = append(row, view.Items[0].Statistics.ViewCount)
		row = append(row, view.VideoId)
		row = append(row, updated)
		rows = append(rows, row)
	}

	copyCount, copyError := conn.CopyFrom(
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

func getContestantViews(contestants map[string][]Contestant) []VideoInfo {
	var contestantViews []VideoInfo
	for _, contestant := range contestants["Contestants"] {
		fmt.Println(contestant.VideoId)
		//TODO: figure out where the empty space comes from
		res, err := http.Get("https://youtube.googleapis.com/youtube/v3/videos?part=statistics&id=" +
			strings.TrimSpace(contestant.VideoId) + "&key=" + os.Getenv("YOUTUBE_API_KEY"))
		if err != nil {
			fmt.Println(err)
		}

		var videoInfo VideoInfo

		body, errBody := io.ReadAll(res.Body)
		if errBody != nil {
			fmt.Println(err)
		}
		if err := json.Unmarshal(body, &videoInfo); err != nil {
			fmt.Println("Failed to unmarshal JSON")
		}

		videoInfo.VideoId = contestant.VideoId
		contestantViews = append(contestantViews, videoInfo)
	}
	return contestantViews
}

func main() {
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var contestants map[string][]Contestant = getContestantsFromDB(conn)

	var contestantViews = getContestantViews(contestants)

	insertViewInfo(conn, contestantViews)
}
