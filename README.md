# WHY

## The world's most useless video player

"Why? is the question you ask yourself when you see this project. Who wants to watch a video in the terminal?

Well, I sometimes I do. Nice for testing stuff on remote VPS's without having to pull them locally. Now also supports
printing jpeg, bmp, and png images to the terminal.


## How do I work?

Turns out its not too hard to draw a picture inside a terminal. The unicode "half block" character (▀) is basically a pixel, so when combined with ansi escape codes, you can individually colorize each one. All video is is images played one after another, so I use ffmpeg to make me a directory of images I then iterate through. To increase display performance, there are actually 2 text windows that alternate being visible, which reduces screen tearing from the text printing across it.


Now with audio support! (I'm not sure if this is the best way to do it, but it works for now)
## Usage
```
  -dl string
        Download a video from YouTube (Video ID)
  -file string
        File to render
  -scale int
        Scale of the image (default 7)

Examples:
./why -file <video> -scale <optional:default 7> 
./why <video>
```

Scaling defaults to 1/7. The number supplied in the command becomes the denominator, e.g. 10 is 1/10.

![image](why.gif)
