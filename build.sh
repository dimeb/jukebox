#!/bin/sh

mkdir -p dist/art
rm -rf dist/templates/*
cp -r -u templates/* dist/templates/.
rm -rf dist/css/*
cp -r -u css/* dist/css/.
rm -rf dist/js/*
cp -r -u js/* dist/js/.
rm -rf dist/img/*
cp -r -u img/* dist/img/.
cp run.sh dist/.
go build -o dist/jukebox
strip dist/jukebox

# GOARM=7 GOARCH=arm GOOS=linux CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc go build -o dist/jukebox_rpi
