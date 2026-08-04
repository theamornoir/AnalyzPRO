package main

import (
	"log"

	"github.com/theamornoir/analyzpro/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	application.Run()
}
