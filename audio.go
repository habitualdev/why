package main

import (
	"context"
	"fmt"
	"github.com/3d0c/gmf"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"log"
	"math"
	"os"
	"time"
)

type AudioFile struct {
	FileName string
	Streamer beep.StreamSeeker
	Format   beep.Format
}

type Player struct {
	File            AudioFile
	CurrentPosition int
	ControlChannel  chan string
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
	newPlayer.CurrentPosition = 0
	if err != nil {
		panic(err)
	}

	return newPlayer
}

func (p Player) Start(ctx context.Context) {
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
			p.CurrentPosition = CurrentPosition
			videoPosition := p.File.Format.SampleRate.N(time.Second) * p.CurrentPosition
			if int(math.Abs(float64(videoPosition-p.File.Streamer.Position()))) > p.File.Format.SampleRate.N(time.Second) {
				speaker.Lock()
				p.File.Streamer.Seek(int(p.File.Format.SampleRate.N(time.Second) * p.CurrentPosition))
				speaker.Unlock()
			}
			continue
		}
	}
}

func VidToAudio(file string) (string, error) {
	/// input
	mic, err := gmf.NewInputCtx(file)
	if err != nil {
		log.Fatalf("Could not open input context: %s", err)
	}
	//mic.Dump()

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

	//outputCtx.Dump()

	count := 0
	for packet := range mic.GetNewPackets() {
		srcFrames, err := cc.Decode(packet)
		packet.Free()
		if err != nil {
			//log.Println("capture audio error:", err)
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
