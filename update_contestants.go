package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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

func GetContestantViewsFromYoutube(contestants map[string][]Contestant) []VideoInfo {
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

func UpdateContestantViews(pool *pgxpool.Pool, event string) {
	var contestants map[string][]Contestant = GetContestantsFromDB(pool, event)

	var contestantViews = GetContestantViewsFromYoutube(contestants)

	InsertViewInfo(pool, contestantViews)
}
