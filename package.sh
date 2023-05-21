# make package dir
if [ ! -d "./package" ]; then
    mkdir ./package
fi

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
