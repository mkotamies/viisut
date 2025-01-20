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
    c.id, c.name, s.view_count, s.updated FROM
    contestant as c
LEFT JOIN LATERAL (
    SELECT s.view_count, s.updated
    FROM statistic as s
    WHERE s.video_id = c.video_id
    ORDER BY s.updated DESC
    LIMIT 1
) s ON true`)

	if err != nil {
		fmt.Println(err)
	}
	for rows.Next() {
		var id int32
		var name string
		var view_count int32
		var updated time.Time

		err := rows.Scan(&id, &name, &view_count, &updated)
		if err != nil {
			fmt.Println(err)
		}
		contestants["Contestants"] = append(contestants["Contestants"], Contestant{Id: strconv.FormatInt(int64(id), 10), Name: name, ViewCount: int(view_count), Updated: updated})
	}
	return contestants
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

	log.Fatal(http.ListenAndServe("localhost:9000", nil))

}
