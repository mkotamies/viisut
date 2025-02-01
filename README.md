# Viisut

## Start locally

- Start locally with live reload `air`
- Start database and pgAdmin locally `docker-compose up -d`
- Insert data from Youtube to database with `go run scripts/contestants.go`

## Build Docker image

docker build -t viisut .

## Run with Docker

docker run -p 9000:9000 viisut

## Deploy with helm

`helm install viisut ./deploy/chart -f ./deploy/chart/values.yaml`
