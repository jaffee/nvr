package main

import (
	"github.com/jaffee/nvr"
	"log"
	"net/http"
)

func main() {
	s := &http.Server{
		Addr:    ":8080",
		Handler: &nvr.VidHandler{Height: 640, Width: 480, BytesPerPixel: 1, FramesPerGif: 200},
	}
	log.Fatalf("server errored: %v", s.ListenAndServe())
}
