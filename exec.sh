#!/bin/bash
if [ ! -d "./bilibili-stickers" ]; then
    # compile
    go build -o bilibili-stickers -ldflags "-s -w" ./
fi

# download emojis
./bilibili-stickers

# make package dir
if [ ! -d "./package" ]; then
    mkdir ./package
fi

# copy executable file
cp ./bilibili-stickers ./package/

stickers=$(ls ./stickers)
for sticker in $stickers; do
    if [ -d ./stickers/$sticker ]; then
        echo $sticker
        if [ ! -f "$sticker.zip" ]; then
            zip -r $sticker.zip ./stickers/$sticker
        fi
        mv $sticker.zip ./package/
    fi
done
