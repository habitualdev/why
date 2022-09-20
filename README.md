# WHY

## The world's most useless video player

"Why? is the question you ask yourself when you see this project. Who wants to watch a video in the terminal?

Well, sometimes I do. Nice for testing stuff on remote VPS without having to pull them locally. Now also supports
printing jpeg, bmp, and png images to the terminal.


## How do I work?

Turns out it's not too hard to draw a picture inside a terminal. The unicode "half block" character (▀) is a *basically* pixel, so when combined with ANSI escape codes, you can individually colourize each one. All video is just a series of images played one after another. I use FFmpeg to make myself a directory of images that  I then iterate through. To increase display performance, 2 text windows alternate in visibility, which greatly reduces screen tearing from the text printing across it.


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
