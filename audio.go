package main

import (
	"context"
	"fmt"
	"github.com/3d0c/gmf"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/jdxyw/generativeart"
	"github.com/jdxyw/generativeart/arts"
	"github.com/jdxyw/generativeart/common"
	tmp3 "github.com/tcolgate/mp3"
	"image/color"
	"io"
	"log"
	"math/rand"
	"os"
	"time"
)

type AudioFile struct {
	FileName string
	Streamer beep.StreamSeeker
	Format   beep.Format
}

type Player struct {
	File           AudioFile
	ControlChannel chan string
	IgnoreSync     bool
}

func NewAudio(file string) Player {

	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	newPlayer := Player{}
	newPlayer.File.FileName = file
	newPlayer.ControlChannel = make(chan string, 1024)
	newPlayer.File.Streamer, newPlayer.File.Format, err = mp3.Decode(f)
	if err != nil {
		log.Println("Unable to decode mp3 file: " + err.Error())
		return Player{}
	}

	return newPlayer
}

func (p Player) Start(ctx context.Context) {
	if p.File.Streamer == nil {
		return
	}
	err := speaker.Init(p.File.Format.SampleRate, p.File.Format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Println("Unable to intialize speakers")
		return
	}
	ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, p.File.Streamer), Paused: false}
	speaker.Play(ctrl)
	for {
		select {
		case <-ctx.Done():
			break
		case command := <-p.ControlChannel:
			speaker.Lock()
			newPos := p.File.Streamer.Position()
			switch command {
			case "pause":
				ctrl.Paused = !ctrl.Paused
			case "back":
				newPos -= p.File.Format.SampleRate.N(time.Second)
				if newPos < 0 {
					newPos = 0
				}
				if newPos >= p.File.Streamer.Len() {
					newPos = p.File.Streamer.Len() - 1
				}
				p.File.Streamer.Seek(newPos)
			case "forward":
				newPos += p.File.Format.SampleRate.N(time.Second)
				if newPos < 0 {
					newPos = 0
				}
				if newPos >= p.File.Streamer.Len() {
					newPos = p.File.Streamer.Len() - 1
				}
				p.File.Streamer.Seek(newPos)
			default:

				continue
			}
			speaker.Unlock()
		default:
			if p.IgnoreSync {
				continue
			}
			continue
		}
	}
}

func VidToAudio(file string) (string, error) {
	gmf.LogSetLevel(gmf.AV_LOG_QUIET)

	mic, err := gmf.NewInputCtx(file)
	if err != nil {
		log.Fatalf("Could not open input context: %s", err)
	}

	ast, err := mic.GetBestStream(gmf.AVMEDIA_TYPE_AUDIO)
	if err != nil {
		log.Fatal("failed to find audio stream")
	}
	cc := ast.CodecCtx()

	/// fifo
	fifo := gmf.NewAVAudioFifo(cc.SampleFmt(), cc.Channels(), 1024)
	if fifo == nil {
		log.Fatal("failed to create audio fifo")
	}

	codec, err := gmf.FindEncoder("libmp3lame")
	if err != nil {
		log.Fatal("find encoder error:", err.Error())
	}

	audioEncCtx := gmf.NewCodecCtx(codec)
	if audioEncCtx == nil {
		log.Fatal("new output codec context error:", err.Error())
	}
	defer audioEncCtx.Free()

	outputCtx, err := gmf.NewOutputCtx("audio.mp3")
	if err != nil {
		log.Fatal("new output fail", err.Error())
		return "", err
	}
	defer outputCtx.Free()

	audioEncCtx.SetSampleFmt(gmf.AV_SAMPLE_FMT_S16P).
		SetSampleRate(cc.SampleRate()).
		SetChannels(cc.Channels()).
		SetBitRate(128e3)

	if outputCtx.IsGlobalHeader() {
		audioEncCtx.SetFlag(gmf.CODEC_FLAG_GLOBAL_HEADER)
	}

	audioStream := outputCtx.NewStream(codec)
	if audioStream == nil {
		log.Fatal(fmt.Errorf("unable to create stream for audioEnc [%s]", codec.LongName()))
	}
	defer audioStream.Free()

	if err := audioEncCtx.Open(nil); err != nil {
		log.Fatal("can't open output codec context", err.Error())
		return "", err
	}
	audioStream.DumpContexCodec(audioEncCtx)

	/// resample
	options := []*gmf.Option{
		{Key: "in_channel_count", Val: cc.Channels()},
		{Key: "out_channel_count", Val: cc.Channels()},
		{Key: "in_sample_rate", Val: cc.SampleRate()},
		{Key: "out_sample_rate", Val: cc.SampleRate()},
		{Key: "in_sample_fmt", Val: cc.SampleFmt()},
		{Key: "out_sample_fmt", Val: gmf.AV_SAMPLE_FMT_S16P},
	}

	swrCtx, err := gmf.NewSwrCtx(options, audioStream.CodecCtx().Channels(), audioStream.CodecCtx().SampleFmt())
	if err != nil {
		log.Fatal("new swr context error:", err.Error())
	}
	if swrCtx == nil {
		log.Fatal("unable to create Swr Context")
	}

	outputCtx.SetStartTime(0)

	if err := outputCtx.WriteHeader(); err != nil {
		log.Fatal(err.Error())
	}

	count := 0
	for packet := range mic.GetNewPackets() {
		srcFrames, err := cc.Decode(packet)
		packet.Free()
		if err != nil {
			continue
		}

		exit := false
		for _, srcFrame := range srcFrames {
			wrote := fifo.Write(srcFrame)
			count += wrote

			for fifo.SamplesToRead() >= 1152 {
				winFrame := fifo.Read(1152)
				dstFrame, err := swrCtx.Convert(winFrame)
				if err != nil {
					log.Println("convert audio error:", err)
					exit = true
					break
				}
				if dstFrame == nil {
					continue
				}
				winFrame.Free()

				writePacket, err := dstFrame.Encode(audioEncCtx)
				if err != nil {
					log.Fatal(err)
				}
				if writePacket == nil {
					continue
				}

				if err := outputCtx.WritePacket(writePacket); err != nil {
					log.Println("write packet err", err.Error())
				}
				writePacket.Free()
				dstFrame.Free()
				if count > int(cc.SampleRate())*10 {
					break
				}
			}
		}
		if exit {
			break
		}
	}
	return "", nil
}

func cmap(r, m1, m2 float64) color.RGBA {
	rgb := color.RGBA{
		uint8(common.Constrain(m1*200*r, 0, 255)),
		uint8(common.Constrain(r*200, 0, 255)),
		uint8(common.Constrain(m2*255*r, 70, 255)),
		255,
	}
	return rgb
}

func GetMp3Length(file string) int {
	t := 0.0

	r, err := os.Open(file)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	d := tmp3.NewDecoder(r)
	var f tmp3.Frame
	skipped := 0

	for {

		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(err)
			return 0
		}

		t = t + f.Duration().Seconds()
	}

	return int(t)
}

func Visualizer() []byte {
	rand.Seed(time.Now().Unix())
	c := generativeart.NewCanva(300, 300)
	colors := []color.RGBA{
		{0xF9, 0xC8, 0x0E, 0xFF},
		{0xF8, 0x66, 0x24, 0xFF},
		{0xEA, 0x35, 0x46, 0xFF},
		{0x66, 0x2E, 0x9B, 0xFF},
		{0x43, 0xBC, 0xCD, 0xFF},
	}
	c.SetBackground(common.Black)
	c.FillBackground()
	c.SetColorSchema(colors)
	c.SetIterations(400)
	c.Draw(arts.NewPixelHole(60))
	imageBytes, _ := c.ToBytes()
	return imageBytes
}
