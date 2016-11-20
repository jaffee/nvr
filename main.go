package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	var urlFilename string
	flag.StringVar(&urlFilename, "urlFile", "camUrls.txt", "file containing urls to read from separated by lines")
	flag.Parse()
	camurl := getCamUrl(urlFilename)

	for {
		tsReader, err := RTSPtoMPEGTS(camurl)
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

func getCamUrl(urlFilename string) string {
	urlFile, err := os.Open(urlFilename)
	if err != nil {
		log.Fatalf("couldn't open url file: %v", err)
	}
	scanner := bufio.NewScanner(urlFile)
	scanner.Scan()
	camurl := scanner.Text()
	if err := scanner.Err(); err != nil {
		log.Fatalf("couldn't read url file: %v", err)
	}

	return camurl
}

func RTSPtoMPEGTS(url string) (io.Reader, error) {
	cmd := exec.Command("ffmpeg",
		"-i", url,
		"-acodec", "copy", "-vcodec", "copy",
		"-f", "mpegts", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("couldn't get stdout pipe from ffmpeg: %v", err)
	}

	br := bufio.NewReader(stdout)

	if err := cmd.Start(); err != nil {
		log.Fatalf("err starting command: %v", err)
	}

	return br, nil
}
