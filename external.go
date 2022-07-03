package main

// Download videos from Youtube

import (
	"github.com/kkdai/youtube/v2"
	"io"
	"os"
)

func DownloadYT(url string) {

	client := youtube.Client{}

	video, err := client.GetVideo(url)
	if err != nil {
		panic(err)
	}
	formats := video.Formats.WithAudioChannels()
	videoIndex := 0
	for n, vid := range formats {
		if vid.ItagNo == 18 {
			videoIndex = n
		}
	}
	stream, _, err := client.GetStream(video, &formats[videoIndex])
	if err != nil {
		panic(err)
	}
	file, err := os.Create("download.mp4")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = io.Copy(file, stream)
	if err != nil {
		panic(err)
	}
}
