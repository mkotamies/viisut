package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
)

type Contestant struct {
	Id    string
	Name string
}

func getContestants(conn *pgx.Conn) map[string][]Contestant{
	var contestants = make(map[string][]Contestant)
	rows, err := conn.Query(context.Background(), "select id, name from contestant")

	if err != nil {
		fmt.Println(err)
	}
	println("vut", err)
	for rows.Next() {
	var id int32
	var name string
	
  	err := rows.Scan(&id, &name)
	if err != nil {
		println(err)
		fmt.Println(err)
	}
	println(id, name)
	contestants["Contestants"] = append(contestants["Contestants"], Contestant{Id: strconv.FormatInt(int64(id), 10), Name: name} )
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
		println("Request received")
		contestants = getContestants(conn) 
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, contestants)
	}


	// define handlers
	http.HandleFunc("/", h1)

	log.Fatal(http.ListenAndServe("localhost:9000", nil))

}
