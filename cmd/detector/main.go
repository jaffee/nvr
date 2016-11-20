package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type GifMaker struct {
	data         []uint8
	frames       int
	framesPerGif int
	gifnum       int
}

func NewGifMaker(framesPerGif int) *GifMaker {
	return &GifMaker{
		data:         make([]uint8, 0),
		framesPerGif: framesPerGif,
	}
}

func (g *GifMaker) Gifify() error {
	defer func() {
		g.frames = 0
		g.data = make([]uint8, 0)
		g.gifnum++
	}()
	td := os.TempDir()
	palCmd := exec.Command("ffmpeg", "-pix_fmt", "gray", "-s:v", "640x480", "-r", "30", "-f", "rawvideo", "-i", "-", "-vf", "fps=15,scale=320:-1:flags=lanczos,palettegen", "-y", td+"palette.png")
	palCmd.Stdout = palCmd.Stderr
	stderrp, _ := palCmd.StderrPipe()
	palIn, err := palCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("palIn stdinPipe: %v", err)
	}
	err = palCmd.Start()
	if err != nil {
		return fmt.Errorf("palCmd start: %v", err)
	}
	n, err := palIn.Write(g.data)
	if err != nil {
		output, errr := ioutil.ReadAll(stderrp)
		log.Printf("readall err: %v", errr)
		log.Println(string(output))
		err := palCmd.Wait()
		log.Printf("wait err: %v", err)
		return fmt.Errorf("palIn.Write: %v, %v", n, err)
	}
	err = palIn.Close()
	if err != nil {
		return fmt.Errorf("palIn.Close: %v", err)
	}
	err = palCmd.Wait()
	if err != nil {
		return fmt.Errorf("palCmd.Wait: %v", err)
	}

	gifCmd := exec.Command("ffmpeg", "-pix_fmt", "gray", "-s:v", "640x480", "-r", "30", "-f", "rawvideo", "-i", "-", "-i", td+"palette.png", "-lavfi", "fps=15,scale=320:-1:flags=lanczos [x]; [x][1:v] paletteuse", "-y", "cam"+strconv.Itoa(g.gifnum)+".gif")
	gifIn, err := gifCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("gifCmd.StdinPipe: %v", err)
	}
	err = gifCmd.Start()
	if err != nil {
		return fmt.Errorf("gifCmd.Start: %v", err)
	}
	_, err = gifIn.Write(g.data)
	if err != nil {
		return fmt.Errorf("gifIn.Write: %v", err)
	}
	err = gifIn.Close()
	if err != nil {
		return fmt.Errorf("gifIn.Close: %v", err)
	}
	err = gifCmd.Wait()
	if err != nil {
		return fmt.Errorf("gifCmd.Wait: %v", err)
	}
	return nil
}

func (g *GifMaker) AddFrame(input []uint8) {
	if g.frames >= g.framesPerGif {
		err := g.Gifify()
		log.Printf("Err gififying: %v", err)
	}
	g.frames += 1
	g.data = append(g.data, input...)
}

type VidHandler struct {
	Height, Width int
	BytesPerPixel int
	FramesPerGif  int
}

func (h *VidHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	edgeCmd := exec.Command("ffmpeg", "-f", "mpegts", "-i", "-",
		"-vf", "scale=640:480,edgedetect",
		"-vcodec", "rawvideo",
		"-pix_fmt", "gray",
		"-f", "rawvideo", "-")
	edgeIn, err1 := edgeCmd.StdinPipe()
	edgeOut, err2 := edgeCmd.StdoutPipe()
	if err3 := edgeCmd.Start(); err1 != nil || err2 != nil || err3 != nil {
		log.Fatalf("Errs: %v, %v, %v", err1, err2, err3)
	}
	rawCmd := exec.Command("ffmpeg", "-f", "mpegts", "-i", "-",
		"-vf", "scale=640:480",
		"-vcodec", "rawvideo",
		"-pix_fmt", "gray",
		"-f", "rawvideo", "-")
	rawIn, err1 := rawCmd.StdinPipe()
	rawOut, err2 := rawCmd.StdoutPipe()
	if err3 := rawCmd.Start(); err1 != nil || err2 != nil || err3 != nil {
		log.Fatalf("Errs: %v, %v, %v", err1, err2, err3)
	}
	streamIn := io.MultiWriter(edgeIn, rawIn)
	go h.handleData(edgeOut, rawOut)
	num, err := io.Copy(streamIn, r.Body)
	log.Printf("Copied %d bytes from packet", num)
	if err != nil {
		log.Printf("Problem copying body to stdin: %v", err)
	}
}

func (h *VidHandler) handleData(edgeOut, rawOut io.Reader) {
	prevEdge, _, err := h.readBoth(edgeOut, rawOut)
	if err != nil {
		log.Printf("err reading both in handleData: %v", err)
		return // TODO
	}
	framesSinceMotion := 100
	gifMaker := NewGifMaker(h.FramesPerGif)
	for {
		nextEdge, nextRaw, err := h.readBoth(edgeOut, rawOut)
		if err != nil {
			log.Printf("err reading both in handleData (loop): %v", err)
			return // TODO
		}
		if h.hasMotion(prevEdge, nextEdge) {
			framesSinceMotion = 0
		} else {
			framesSinceMotion++
		}
		if framesSinceMotion < 20 {
			gifMaker.AddFrame(nextRaw)
		} else if framesSinceMotion == 20 {
			// need to reset gifmaker
			err = gifMaker.Gifify()
			log.Printf("err gififying in reset: %v", err)
		}
		prevEdge = nextEdge
	}

}

func (h *VidHandler) hasMotion(prev, next []uint8) bool {
	if len(prev) != len(next) {
		panic("non matching lengths in hasMotion")
	}
	changed_count := 0
	for i := 0; i < len(prev); i++ {
		if (prev[i] == 0 && next[i] != 0) || (prev[i] != 0 && next[i] == 0) {
			changed_count += 1
		}
	}
	return changed_count > len(prev)/100
}

func (h *VidHandler) readBoth(edgeOut, rawOut io.Reader) (edgeFrame, rawFrame []uint8, err error) {
	edge := make([]uint8, h.Width*h.Height*h.BytesPerPixel)
	raw := make([]uint8, h.Width*h.Height*h.BytesPerPixel)
	_, err = io.ReadFull(edgeOut, edge)
	if err != nil {
		return nil, nil, err
	}
	_, err = io.ReadFull(rawOut, raw)
	if err != nil {
		return nil, nil, err
	}
	return edge, raw, nil
}

func main() {
	s := &http.Server{
		Addr:    ":8080",
		Handler: &VidHandler{Height: 640, Width: 480, BytesPerPixel: 1, FramesPerGif: 300},
	}
	log.Fatalf("server errored: %v", s.ListenAndServe())
}
