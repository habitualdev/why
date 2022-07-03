package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	_ "golang.org/x/image/bmp"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
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

var imageMagic = [][]byte{{0x89, 0x50, 0x4E, 0x47, 0x0D}, {0x42, 0x4D}, {0xFF, 0xD8, 0xFF, 0xDB}, {0xFF, 0xD8, 0xFF, 0xE0},
	{0xFF, 0xD8, 0xFF, 0xEE}, {0xFF, 0xD8, 0xFF, 0xE1}, {0xFF, 0xD8, 0xFF, 0xE0}}

var size int
var i int
var paused bool
var skip int
var CurrentPosition int

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

func renderPicture(picture []byte) string {
	if skip == 0 {
		skip = 7
	}
	img := openImage(picture)
	str := convertImageToANSI(img, skip)
	return str
}

// ExtractFrames Legacy frame extractor, uses ffmpeg to extract frames
func ExtractFrames(filename string) {
	// create the frames directory
	os.Mkdir("frames", 0777)
	// extract the frames
	c := exec.Command("ffmpeg", "-i", filename, "-filter:v", "fps=30", "frames/%d.jpg")
	c.Run()
}

func main() {
	var scale = flag.Int("scale", 7, "Scale of the image")
	var file = flag.String("file", "", "File to render")
	var dl = flag.String("dl", "", "Download a video from Youtube")
	var mediaType string
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx.Done()
	flag.Parse()

	if *file == "" {
		*file = os.Args[1]
	}

	if *dl != "" {
		println("Downloading video...")
		*file = "download.mp4"
		DownloadYT(*dl)
		println("Video downloaded!")
	}

	skip = *scale
	boxNum := 0
	paused = false

	os.RemoveAll("frames")
	os.Mkdir("frames", 0777)

	if _, err := os.Stat(*file); err != nil {
		if os.IsNotExist(err) {
			log.Fatal("File does not exist")
		}
		os.Exit(1)
	}

	data, _ := os.ReadFile(*file)
	for _, magic := range imageMagic {
		if bytes.Contains(data[0:16], magic) {
			mediaType = "image"
			break
		} else {
			mediaType = "video"
		}
	}

	if mediaType == "image" {
		skip = *scale
		fmt.Println(renderPicture(data))
		os.Exit(0)
	} else if mediaType == "video" {
		go ExtractImages(*file, ctx)
		VidToAudio(*file)

		audioPlayer := NewAudio("audio.mp3")

		extractCheck := true
		for extractCheck {
			if _, err := os.Stat("frames/1.jpg"); err == nil {
				extractCheck = false
				time.Sleep(100 * time.Millisecond)
			}
		}
		app := tview.NewApplication()
		box := tview.NewTextView().SetDynamicColors(true)
		box2 := tview.NewTextView().SetDynamicColors(true)
		box2.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Rune() == 'd' {
				i = i + 24
				if i >= size {
					i = size - 1
				}
				audioPlayer.ControlChannel <- "forward"
			}
			if event.Rune() == 'a' {
				i = i - 24
				if i < 0 {
					i = 0
				}
				audioPlayer.ControlChannel <- "back"
			}
			if event.Rune() == ' ' {
				paused = !paused
				audioPlayer.ControlChannel <- "pause"
			}
			if event.Rune() == 'q' {
				cancel()
				wd, _ := os.Getwd()
				os.RemoveAll(wd + "/frames")
				if *dl != "" {
					os.Remove("download.mp4")
				}
				if _, err := os.Stat("audio.mp3"); err == nil {
					os.Remove("audio.mp3")
				}
				app.Stop()
				os.Exit(0)
			}
			if event.Rune() == 'f' {
				skip--
				if skip < 1 {
					skip = 1
				}
			}
			if event.Rune() == 'r' {
				skip++
				if skip > 10 {
					skip = 10
				}
			}
			return event
		})
		box.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Rune() == 'd' {
				i = i + 24
				if i >= size {
					i = size - 1
				}
				audioPlayer.ControlChannel <- "forward"
			}
			if event.Rune() == 'a' {
				i = i - 24
				if i < 0 {
					i = 0
				}
				audioPlayer.ControlChannel <- "back"
			}
			if event.Rune() == ' ' {
				paused = !paused
				audioPlayer.ControlChannel <- "pause"
			}
			if event.Rune() == 'q' {
				cancel()
				wd, _ := os.Getwd()
				os.RemoveAll(wd + "/frames")
				if *dl != "" {
					os.Remove("download.mp4")
				}
				if _, err := os.Stat("audio.mp3"); err == nil {
					os.Remove("audio.mp3")
				}
				app.Stop()
				os.Exit(0)
			}
			if event.Rune() == 'f' {
				skip--
				if skip < 1 {
					skip = 1
				}
			}
			if event.Rune() == 'r' {
				skip++
				if skip > 10 {
					skip = 10
				}
			}
			return event
		})
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		box2.SetText("Loading...")
		box.SetText("Loading...")
		go audioPlayer.Start(ctx)
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
								text := renderPicture(buf)
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
								box.SetText(ansi + "  " + spacert + secondsToMinutes(i/24) + "/" + secondsToMinutes(size/24) +
									"\n" + spacerb + "<--- 'a' | spacebar |  'd' --->  |  'q'   |   'f'    |   'r'" +
									"\n" + spacerb + " Rewind  |   pause  |  Fast Fwd  |  quit  | scale ▲  | scale ▼")
								boxNum = 1
								app.SetRoot(box2, true)
							})
					} else {
						app.QueueUpdateDraw(
							func() {
								text := renderPicture(buf)
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
								box2.SetText(ansi + "  " + spacert + secondsToMinutes(i/24) + "/" + secondsToMinutes(size/24) +
									"\n" + spacerb + "<--- 'a' | spacebar |  'd' --->  |  'q'   |   'f'    |   'r'" +
									"\n" + spacerb + " Rewind  |   pause  |  Fast Fwd  |  quit  | scale ▲  | scale ▼")
								boxNum = 0
								app.SetRoot(box, true)
							})
					}
					for time.Now().Sub(start) < (40 * time.Millisecond) {
						time.Sleep(1 * time.Millisecond)
					}
					CurrentPosition = i / 24
					i++
				}
			}
		}()
		<-c
		app.Stop()
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}
}
