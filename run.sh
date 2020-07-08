#!/bin/bash

declare -a DRIVES
export DRIVES
declare -a MOUNTED
export MOUNTED
function exit_handler_mount() {
  for d in ${MOUNTED[@]}; do
    pumount /mnt/$d
  done
}

MACHINE=`uname -m`
if [[ $MACHINE == arm* ]]; then
  if [[ -z "${SSH_TTY}" ]]; then
    lsblk --noheadings --raw -o NAME,TYPE,MOUNTPOINT | grep '^sd[a-z][0-9] part $' | cut -d " " -f 1 | while read drive ; do
      mkdir -p /mnt/$drive
      pmount /dev/$drive /mnt/$drive
      if [ $? -eq 0 ] && [ -d /mnt/$drive/Music ]; then
        $MOUNTED+=( $drive )
        $DRIVES+=( /mnt/$drive/Music )
      fi
    done
  else
    exit 0
  fi
else
  if [ -d $HOME/Music ]; then
    $DRIVES+=( $HOME/Music )
  fi 
fi
export RCLONE_CONFIG="--config rclone.config"
rclone listremotes $RCLONE_CONFIG | grep ^Local | cut -d ":" -f 1 | while read remote ; do
  rclone config delete $remote $RCLONE_CONFIG
done
for i in ${!DRIVES[@]}; do
  mkdir -p Music/${DRIVES[$i]}
  rclone config create Local$i alias remote ${DRIVES[$i]} $RCLONE_CONFIG
done

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

JUKEBOX_KIOSK_USER_DATA_DIR=$PWD/kiosk
mkdir -p $JUKEBOX_KIOSK_USER_DATA_DIR

EXE=
for exe in \
  /usr/bin/chrome \
  /opt/google/chrome/chrome \
  /usr/bin/chromium \
  /usr/bin/chromium-browser
do
  if [ -x $exe ]; then
    EXE="$exe --kiosk --incognito --noerrdialogs --disable-infobars --disable-gpu --no-default-browser-check --user-data-dir=$JUKEBOX_KIOSK_USER_DATA_DIR $JUKEBOX_DISPLAY http://$JUKEBOX_KIOSK/app"
    break
  fi
done

if [[ $MACHINE == arm* ]]; then
  mkdir -p $HOME/.config/openbox
  AUTOSTART=$HOME/.config/openbox/autostart
  echo -e "xset -dpms\nxset s noblank\nxset s off\nsetxkbmap -option terminate:ctrl_alt_bksp" > $AUTOSTART
  echo -e "sed -i 's/\"exited_cleanly\":false/\"exited_cleanly\":true/' ~/.config/chromium/'Local State'" >> $AUTOSTART
  echo -e "sed -i 's/\"exited_cleanly\":false/\"exited_cleanly\":true/; s/\"exit_type\":\"[^\"]\+\"/\"exit_type\":\"Normal\"/' ~/.config/chromium/Default/Preferences" >> $AUTOSTART
  echo $EXE >> $AUTOSTART
  startx -- -nocursor
else
  $EXE
fi

