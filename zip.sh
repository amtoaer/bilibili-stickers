#!/bin/bash

if [ ! -d "./package" ]; then
    mkdir ./package
fi

cd ./stickers

stickers=$(ls ./)
for sticker in $stickers; do
    if [ -d $sticker ]; then
        echo $sticker
        if [ ! -f "$sticker.zip" ]; then
            zip -r $sticker.zip $sticker
        fi
        mv $sticker.zip ../package/
    fi
done
