#!/bin/bash
if [ ! -d "./bilibili-stickers" ]; then
    # compile
    go build -o bilibili-stickers -ldflags "-s -w" ./
fi

# download emojis
./bilibili-stickers