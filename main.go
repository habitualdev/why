package main

import (
	"bytes"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const UPPER_HALF_BLOCK = "▀"

var size int
var i int
var paused bool

func secondsToMinutes(inSeconds int) string {
	minutes := inSeconds / 60
	seconds := inSeconds % 60
	str := fmt.Sprintf("%d:%d", minutes, seconds)
	return str
}

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
		log.Println(err)
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
	// create the frames directory
	os.Mkdir("frames", 0777)
	// extract the frames
	c := exec.Command("ffmpeg", "-i", filename, "-filter:v", "fps=30", "frames/%d.jpg")
	c.Run()
}

func main() {
	skip := 0
	boxNum := 0
	paused = false
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

	os.RemoveAll("frames")

	go ExtractFrames(os.Args[1])

	extractCheck := true

	for extractCheck {
		if _, err := os.Stat("frames/1.jpg"); err == nil {
			extractCheck = false
		}
	}

	app := tview.NewApplication()
	box := tview.NewTextView().SetDynamicColors(true)
	box2 := tview.NewTextView().SetDynamicColors(true)

	box2.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'd' {
			i = i + 30
			if i >= size {
				i = size - 1
			}
		}
		if event.Rune() == 'a' {
			i = i - 30
			if i < 0 {
				i = 0
			}
		}
		if event.Rune() == ' ' {
			paused = !paused
		}
		return event
	})

	box.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'd' {
			i = i + 30
			if i >= size {
				i = size - 1
			}
		}
		if event.Rune() == 'a' {
			i = i - 30
			if i < 0 {
				i = 0
			}
		}
		if event.Rune() == ' ' {
			paused = !paused
		}
		return event
	})

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	box2.SetText("Loading...")
	box.SetText("Loading...")

	go func() {
		for {
			files, err := os.ReadDir("frames")
			if err != nil {
				log.Fatal(err)
			}
			size = len(files)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		loopCheck := true

		go app.SetRoot(box, true).Run()
		i = 1
		for loopCheck {
			if paused {
				continue
			}
			if i != size {
				start := time.Now()
				buf, _ := os.ReadFile("frames/" + strconv.Itoa(i) + ".jpg")
				if boxNum == 0 {
					app.QueueUpdateDraw(
						func() {
							text := renderPicture(buf, skip)
							w := len(strings.Split(text, "\n")[0])
							spaceb := w - 37
							if spaceb < 0 {
								spaceb = 0
							}
							spacet := w - 10
							if spacet < 0 {
								spacet = 0
							}
							spacert := strings.Repeat(" ", spacet/100)
							spacerb := strings.Repeat(" ", spaceb/100)
							ansi := tview.TranslateANSI(text)
							box.SetText(ansi + "  " + spacert + secondsToMinutes(i/30) + "/" + secondsToMinutes(size/30) +
								"\n" + spacerb + "<--- 'a' | spacebar:pause |  'd' --->")
							boxNum = 1
							app.SetRoot(box2, true)
						})
				} else {
					app.QueueUpdateDraw(
						func() {
							text := renderPicture(buf, skip)
							w := len(strings.Split(text, "\n")[0])
							spaceb := w - 37
							if spaceb < 0 {
								spaceb = 0
							}
							spacet := w - 10
							if spacet < 0 {
								spacet = 0
							}
							spacert := strings.Repeat(" ", spacet/100)
							spacerb := strings.Repeat(" ", spaceb/100)
							ansi := tview.TranslateANSI(text)
							box2.SetText(ansi + "  " + spacert + secondsToMinutes(i/30) + "/" + secondsToMinutes(size/30) +
								"\n" + spacerb + "<--- 'a' | spacebar:pause |  'd' --->")
							boxNum = 0
							app.SetRoot(box, true)
						})
				}
				for time.Now().Sub(start) < (33 * time.Millisecond) {
					time.Sleep(1 * time.Millisecond)
				}
				i++
			}
		}
	}()
	<-c
	app.Stop()
	os.Exit(0)
}
