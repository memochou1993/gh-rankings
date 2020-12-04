package main

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/memochou1993/github-rankings/app"
	"log"
)

func main() {
	database := app.Database{}

	if err := database.CollectUsers(); err != nil {
		log.Println(err.Error())
	}
}
