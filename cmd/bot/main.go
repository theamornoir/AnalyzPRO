package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/theamornoir/analyzpro/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("🚀 Prisma запускается...")

	application.Run(ctx)
}
