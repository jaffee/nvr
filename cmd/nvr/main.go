package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/jaffee/nvr"
)

func main() {
	var urlFilename string
	flag.StringVar(&urlFilename, "urlFile", "camUrls.txt", "file containing urls to read from separated by lines")
	flag.Parse()
	camurl := nvr.GetCamUrl(urlFilename)

	for {
		tsReader, err := nvr.RTSPtoMPEGTS(camurl)
		if err != nil {
			log.Fatal(err)
		}
		// const packetLen = 188
		// pkt := make([]byte, packetLen)

		resp, err := http.Post("http://localhost:8080", "application/mpegts", tsReader)
		if err != nil {
			log.Printf("%v, %v", resp, err)
		}
		log.Println(resp)
		time.Sleep(time.Second)
	}

}
