#!/bin/bash

RUNLOGFILE=run.log

JUKEBOX_MOUNT=""
export JUKEBOX_LOCAL_DRIVE=""
export JUKEBOX_MUSIC_DIR="Music"
export JUKEBOX_MOUNT_POINT="$JUKEBOX_MUSIC_DIR/Local"

function exit_handler_mount() {
  if [[ -n "$JUKEBOX_MOUNT" ]]; then
    pumount /dev/$JUKEBOX_MOUNT >>$RUNLOGFILE 2>&1
  fi
  rm $JUKEBOX_MOUNT_POINT >>$RUNLOGFILE 2>&1
}

mkdir -p $JUKEBOX_MUSIC_DIR

rm -rf $JUKEBOX_MOUNT_POINT
if [[ -a $JUKEBOX_MOUNT_POINT ]]; then
  echo "Cannot remove $JUKEBOX_MOUNT_POINT" >> $RUNLOGFILE
  exit 1
fi

MACHINE=`uname -m`

if [[ -z "${SSH_TTY}" ]]; then
  if [[ $MACHINE == arm* ]]; then
    param=`lsblk --noheadings --raw -o NAME,TYPE,MOUNTPOINT,HOTPLUG | grep '^sd[a-z][0-9] part .* 1$' | head -n 1`
    if [ ${#param[@]} -eq 3 ]; then
      pmount -r /dev/${param[0]} >>$RUNLOGFILE 2>&1
      if [ $? -eq 0 ]; then
        JUKEBOX_MOUNT="${param[0]}"
        JUKEBOX_LOCAL_DRIVE="/media/${param[0]}"
      fi
    else
      JUKEBOX_LOCAL_DRIVE="${param[2]}"
    fi
  else
    if [ -d $HOME/Music ] && [ -n "$(ls -A $HOME/Music)" ]; then
      JUKEBOX_LOCAL_DRIVE="$HOME/Music"
    fi
  fi
else
  exit 0
fi

if [[ -n "$JUKEBOX_LOCAL_DRIVE" ]]; then
  ln -s $JUKEBOX_LOCAL_DRIVE $JUKEBOX_MOUNT_POINT >>$RUNLOGFILE 2>&1
else
  echo "Cannot find local music drive" >> $RUNLOGFILE
  exit 2
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

