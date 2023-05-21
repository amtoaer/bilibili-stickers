#!/bin/bash
if [ ! -d "./sticker-downloader" ]; then
    # compile
    go build -o sticker-downloader -ldflags "-s -w" ./
fi

# download emojis
./sticker-downloader

# make package dir
if [ ! -d "./package" ]; then
    mkdir ./package
fi

# copy executable file
mv ./sticker-downloader ./package/

# zip stickers file
zip -r ./package/stickers.zip ./stickers/

