package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, _ := sql.Open("sqlite3", "db.db")

	var f float64
	err := db.QueryRow("SELECT 0.240066230297088;").Scan(&f)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(f)
}
