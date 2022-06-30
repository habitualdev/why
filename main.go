package main

import (
	"bytes"
	"fmt"
	"github.com/rivo/tview"
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const UPPER_HALF_BLOCK = "▀"

// 48;2;r;g;bm - set background colour to rgb
func rgbBackgroundSequence(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// 38;2;r;g;bm - set text colour to rgb
func rgbTextSequence(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func resetColorSequence() string {
	return "\x1b[0m"
}

func convertColorToRGB(col color.Color) (uint8, uint8, uint8) {
	rgbaColor := color.RGBAModel.Convert(col)
	_r, _g, _b, _ := rgbaColor.RGBA()
	// rgb values are uint8s, I cannot comprehend why the stdlib would return
	// int32s :facepalm:
	r := uint8(_r & 0xFF)
	g := uint8(_g & 0xFF)
	b := uint8(_b & 0xFF)
	return r, g, b
}

func convertImageToANSI(img image.Image, skip int) string {
	// We'll just reuse this to increment the loop counters
	skip += 1
	ansi := resetColorSequence()
	yMax := img.Bounds().Max.Y
	xMax := img.Bounds().Max.X

	sequences := make([]string, yMax)

	for y := img.Bounds().Min.Y; y < yMax; y += 2 * skip {
		sequence := ""
		for x := img.Bounds().Min.X; x < xMax; x += skip {
			upperPix := img.At(x, y)
			lowerPix := img.At(x, y+skip)

			ur, ug, ub := convertColorToRGB(upperPix)
			lr, lg, lb := convertColorToRGB(lowerPix)

			if y+skip >= yMax {
				sequence += resetColorSequence()
			} else {
				sequence += rgbBackgroundSequence(lr, lg, lb)
			}

			sequence += rgbTextSequence(ur, ug, ub)
			sequence += UPPER_HALF_BLOCK

			sequences[y] = sequence
		}
	}

	for y := img.Bounds().Min.Y; y < yMax; y += 2 * skip {
		ansi += sequences[y] + resetColorSequence() + "\n"
	}

	return ansi
}

func openImage(picture []byte) image.Image {
	reader := bytes.NewReader(picture)
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}

	return img
}

func renderPicture(picture []byte, skip int) string {
	if skip == 0 {
		skip = 7
	}
	img := openImage(picture)
	str := convertImageToANSI(img, skip)
	return str
}

func ExtractFrames(filename string) {
	// if the frames directory exists, delete it
	if _, err := os.Stat("frames"); err == nil {
		os.RemoveAll("frames")
	}
	// create the frames directory
	os.Mkdir("frames", 0777)
	// extract the frames
	c := exec.Command("ffmpeg", "-i", filename, "-filter:v", "fps=30", "frames/%d.jpg")
	c.Run()
}

func main() {
	skip := 0
	boxNum := 0
	// if filename doesn't exist, exit
	if len(os.Args) < 2 {
		log.Fatal("Please provide a filename")
	}

	if len(os.Args) > 1 {
		if _, err := os.Stat(os.Args[1]); err != nil {
			log.Fatal("File not found")
		}
		if len(os.Args) > 2 {
			if n, err := strconv.Atoi(os.Args[2]); err == nil {
				skip = n
			} else {
				log.Fatal("Invalid skip value")
			}
		}
	}

	ExtractFrames(os.Args[1])
	app := tview.NewApplication()
	box := tview.NewTextView().SetDynamicColors(true)
	box2 := tview.NewTextView().SetDynamicColors(true)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	box2.SetText("Loading...")
	box.SetText("Loading...")

	files, err := os.ReadDir("frames")
	if err != nil {
		log.Fatal(err)
	}

	size := len(files)

	go func() {
		go app.SetRoot(box, true).Run()
		for i := 1; i <= size; i++ {
			start := time.Now()
			buf, err := os.ReadFile("frames/" + strconv.Itoa(i) + ".jpg")
			if err != nil {
				log.Fatal(err)
			}
			if boxNum == 0 {
				app.QueueUpdateDraw(
					func() {
						text := renderPicture(buf, skip)
						ansi := tview.TranslateANSI(text)
						box.SetText(ansi + "\n" + strconv.Itoa(i) + "/" + strconv.Itoa(size) + " frames")
						boxNum = 1
						app.SetRoot(box2, true)
					})
			} else {
				app.QueueUpdateDraw(
					func() {
						text := renderPicture(buf, skip)
						ansi := tview.TranslateANSI(text)
						box2.SetText(ansi + "\n" + strconv.Itoa(i) + "/" + strconv.Itoa(size) + " frames")
						boxNum = 0
						app.SetRoot(box, true)
					})
			}
			for time.Now().Sub(start) < (33 * time.Millisecond) {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()
	<-c
	app.Stop()
	os.Exit(0)
}
