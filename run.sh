#!/bin/bash

export LOCAL_DIR=Music/Local
mkdir -p $PWD/$LOCAL_DIR

MACHINE=`uname -m`
if [[ $MACHINE == arm* ]]; then
  if [[ -z "${SSH_TTY}" ]]; then
    pumount /dev/sda1
    pmount /dev/sda1 $PWD/$LOCAL_DIR || exit 1
  else
    exit 0
  fi
  function exit_handler_mount { pumount /dev/sda1; }
else
  sudo mount --bind /home/dime/Music/ $PWD/$LOCAL_DIR || exit 1
  function exit_handler_mount { sudo umount $PWD/$LOCAL_DIR; }
fi

export JUKEBOX=./jukebox
export JUKEBOX_ERR_FILE=./jukebox.err
export JUKEBOX_ENV_FILE=./jukebox.env

rm $JUKEBOX_ENV_FILE

$JUKEBOX 1>> $JUKEBOX_ERR_FILE 2>> $JUKEBOX_ERR_FILE &
export JUKEBOX_PID=$!

function exit_handler_jukebox { kill -9 $JUKEBOX_PID; }

trap "exit_handler_mount; exit_handler_jukebox" EXIT

while [[ ! -f $JUKEBOX_ENV_FILE ]]; do sleep 1s; done

source $JUKEBOX_ENV_FILE

if [[ $1 == noui ]]; then
  exit
fi

JUKEBOX_DISPLAY=
JUKEBOX_MONITORS=`xrandr --listmonitors | grep  '^ *[0-9]:' | wc -l`
if [ "$JUKEBOX_MONITORS" != "1" ]; then
  JUKEBOX_DISPLAY="--window-position=1920,0"
fi

EXE=
for exe in \
  chrome \
  /opt/google/chrome/chrome \
  chromium \
  chromium-browser
do
  if [ -x "$(command -v ${exe})" ]; then
    EXE=$exe
    break
  fi
done

JUKEBOX_KIOSK_USER_DATA_DIR=$PWD/kiosk
mkdir -p $JUKEBOX_KIOSK_USER_DATA_DIR
$EXE \
  --kiosk \
  --incognito \
  --noerrdialogs \
  --disable-infobars \
  --disable-gpu \
  --no-default-browser-check \
  --user-data-dir=$JUKEBOX_KIOSK_USER_DATA_DIR \
  $JUKEBOX_DISPLAY http://$JUKEBOX_KIOSK/app

