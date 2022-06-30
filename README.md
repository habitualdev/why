# WHY

## The world's most useless video player

"Why? is the question you ask yourself when you see this project. Who wants to watch a video in the terminal?

Well, I sometimes I do. Nice for testing stuff on remote VPS's without having to pull them locally.

Requires ffmpeg, currently only works on linux (tested on ubuntu 22.04)

## How do I work?

Turns out its not too hard to draw a picture inside a terminal. The unicode "half block" character (▀) is basically a pixel, so when combined with ansi escape codes, you can individually colorize each one. All video is is images played one after another, so I use ffmpeg to make me a directory of images I then iterate through. To increase display performance, there are actually 2 text windows that alternate being visible, which reduces screen tearing from the text printing across it.

## Usage
```
./why <video> <optional:scaling>
```

Scaling defaults to 1/7. The number supplied in the command becomes the denominator, e.g. 10 is 1/10.

![image](why.gif)
